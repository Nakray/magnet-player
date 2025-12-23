package storage

import (
	"encoding/json"
	"time"

	bolt "go.etcd.io/bbolt"
)

type MetadataDB struct {
	db *bolt.DB
}

const bucketName = "FileCache"

func NewMetadataDB(path string) (*MetadataDB, error) {
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, e := tx.CreateBucketIfNotExists([]byte(bucketName))
		return e
	})
	if err != nil {
		return nil, err
	}
	return &MetadataDB{db: db}, nil
}

func (m *MetadataDB) Close() error {
	return m.db.Close()
}

func (m *MetadataDB) Save(meta *FileMeta) error {
	return m.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		data, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		return b.Put([]byte(meta.Hash), data)
	})
}

func (m *MetadataDB) GetAllFiles() ([]*FileMeta, error) {
	var files []*FileMeta

	err := m.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil // кеш пустой
		}

		return b.ForEach(func(k, v []byte) error {
			var fm FileMeta
			if err := json.Unmarshal(v, &fm); err != nil {
				return nil // ошибки просто скипаем
			}
			files = append(files, &fm)
			return nil
		})
	})
	return files, err
}
