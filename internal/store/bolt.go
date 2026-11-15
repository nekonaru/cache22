package store

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	bucketDrops = []byte("drops")
	bucketMeta  = []byte("meta")
	keyUsed     = []byte("used_bytes")
)

// Bolt is a persistent Store backed by bbolt. Reads and writes happen inside
// a single bbolt transaction, so Consume is atomic across restarts and across
// any concurrent clients attached to the same process.
//
// The file is created with owner-only permissions because confidential
// material (ciphertext and passphrase-wrapped keys) lives on disk.
type Bolt struct {
	db       *bolt.DB
	maxBytes int64
	path     string
}

// NewBolt opens (or creates) a bolt database at path, bounded to maxBytes of
// decoded payload data. Opening an existing, in-use file fails loudly.
func NewBolt(path string, maxBytes int64) (*Bolt, error) {
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open bolt %s: %w", path, err)
	}
	if fi, err := os.Stat(path); err == nil {
		// Best effort: on Windows this is a no-op, which is fine.
		_ = os.Chmod(path, 0o600)
		_ = fi
	}
	b := &Bolt{db: db, maxBytes: maxBytes, path: path}
	if err := db.Update(func(tx *bolt.Tx) error {
		for _, name := range [][]byte{bucketDrops, bucketMeta} {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init bolt buckets: %w", err)
	}
	return b, nil
}

// Close flushes and closes the database file. Safe to call multiple times.
func (b *Bolt) Close() error {
	if b.db == nil {
		return nil
	}
	err := b.db.Close()
	b.db = nil
	return err
}

// Path reports where the data file lives (useful in error messages and tests).
func (b *Bolt) Path() string { return b.path }

type dropRecord struct {
	ID        string `json:"id"`
	Payload   string `json:"payload"`
	Kind      string `json:"kind"`
	Meta      []byte `json:"meta,omitempty"`
	ReadsLeft int    `json:"reads_left"`
	ExpiresAt int64  `json:"expires_at"`
	CreatedAt int64  `json:"created_at"`
	Size      int    `json:"size"`
}

func (b *Bolt) Put(d Drop) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		used := readUsed(tx)
		if b.maxBytes > 0 && used+int64(d.size) > b.maxBytes {
			return ErrFull
		}
		rec := dropRecord{
			ID: d.ID, Payload: d.Payload, Kind: d.Kind, Meta: d.Meta,
			ReadsLeft: d.ReadsLeft, ExpiresAt: d.ExpiresAt.UnixNano(),
			CreatedAt: d.CreatedAt.UnixNano(), Size: d.size,
		}
		buf, err := json.Marshal(rec)
		if err != nil {
			return err
		}
		if err := tx.Bucket(bucketDrops).Put([]byte(d.ID), buf); err != nil {
			return err
		}
		return writeUsed(tx, used+int64(d.size))
	})
}

func (b *Bolt) Consume(id string, now time.Time) (Drop, bool) {
	var out Drop
	var ok bool
	_ = b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketDrops)
		v := bucket.Get([]byte(id))
		if v == nil {
			return nil
		}
		var rec dropRecord
		if err := json.Unmarshal(v, &rec); err != nil {
			return err
		}
		d := rec.toDrop()
		if !now.Before(d.ExpiresAt) {
			_ = bucket.Delete([]byte(id))
			_ = writeUsed(tx, readUsed(tx)-int64(rec.Size))
			return nil
		}
		d.ReadsLeft--
		out, ok = d, true
		if d.ReadsLeft <= 0 {
			_ = bucket.Delete([]byte(id))
			_ = writeUsed(tx, readUsed(tx)-int64(rec.Size))
			return nil
		}
		buf, err := json.Marshal(dropRecord{
			ID: d.ID, Payload: d.Payload, Kind: d.Kind, Meta: d.Meta,
			ReadsLeft: d.ReadsLeft, ExpiresAt: d.ExpiresAt.UnixNano(),
			CreatedAt: d.CreatedAt.UnixNano(), Size: rec.Size,
		})
		if err != nil {
			return err
		}
		return bucket.Put([]byte(d.ID), buf)
	})
	return out, ok
}

func (b *Bolt) Sweep(now time.Time) int {
	n := 0
	_ = b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketDrops)
		var expired [][]byte
		used := readUsed(tx)
		_ = bucket.ForEach(func(k, v []byte) error {
			var rec dropRecord
			if err := json.Unmarshal(v, &rec); err != nil {
				return nil // skip undecodable record rather than abort the sweep
			}
			if !now.Before(time.Unix(0, rec.ExpiresAt)) {
				expired = append(expired, append([]byte{}, k...))
			}
			return nil
		})
		for _, k := range expired {
			v := bucket.Get(k)
			var rec dropRecord
			if v != nil && json.Unmarshal(v, &rec) == nil {
				used -= int64(rec.Size)
			}
			_ = bucket.Delete(k)
			n++
		}
		if len(expired) > 0 {
			_ = writeUsed(tx, used)
		}
		return nil
	})
	return n
}

func readUsed(tx *bolt.Tx) int64 {
	v := tx.Bucket(bucketMeta).Get(keyUsed)
	if v == nil {
		return 0
	}
	var n int64
	_, _ = fmt.Sscanf(string(v), "%d", &n)
	return n
}

func writeUsed(tx *bolt.Tx, n int64) error {
	if n < 0 {
		n = 0
	}
	return tx.Bucket(bucketMeta).Put(keyUsed, []byte(fmt.Sprintf("%d", n)))
}

func (r dropRecord) toDrop() Drop {
	return Drop{
		ID:        r.ID,
		Payload:   r.Payload,
		Kind:      r.Kind,
		Meta:      append([]byte(nil), r.Meta...),
		ReadsLeft: r.ReadsLeft,
		ExpiresAt: time.Unix(0, r.ExpiresAt).UTC(),
		CreatedAt: time.Unix(0, r.CreatedAt).UTC(),
		size:      r.Size,
	}
}
