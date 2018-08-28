package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"time"
	
	"github.com/boltdb/bolt"
)

// MetaStore implements a metadata storage. It stores user credentials and Meta information
// for objects. The storage is handled by boltdb.
type BoltMetaStore struct {
	db *bolt.DB
}

var (
	errNoBucket       = errors.New("Bucket not found")
	errObjectNotFound = errors.New("Object not found")
	errNotOwner       = errors.New("Attempt to delete other user's lock")
	objectsBucket = []byte("objects")
)

// NewMetaStore creates a new MetaStore using the boltdb database at dbFile.
func NewBoltMetaStore(dbFile string) (*BoltMetaStore, error) {
	db, err := bolt.Open(dbFile, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}

	db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(objectsBucket); err != nil {
			return err
		}
		return nil
	})
	return &BoltMetaStore{db: db}, nil
}

func (s *BoltMetaStore) Get(hash string) (*MetaData, error) {
	var d MetaData
	
	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(objectsBucket)
		if bucket == nil {
			return errNoBucket
		}
		
		value := bucket.Get([]byte(hash))
		if len(value) == 0 {
			return errObjectNotFound
		}
		
		dec := gob.NewDecoder(bytes.NewBuffer(value))
		return dec.Decode(&d)
	})
	
	if err != nil {
		return nil, err
	}
	
	return &d, nil
}

// Put writes meta information to the store, keyed by the object hash.
func (s *BoltMetaStore) Put(hash string, d *MetaData) error {	// Check if it exists first
	// Check if it exists first
	if _, err := s.Get(hash); err == nil {
		return nil
	}
	
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(d)
	if err != nil {
		return err
	}
	
	err = s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(objectsBucket)
		if bucket == nil {
			return errNoBucket
		}
		
		err = bucket.Put([]byte(hash), buf.Bytes())
		if err != nil {
			return err
		}
	
		return nil
	})
	
	if err != nil {
		return err
	}
	
	return nil
}

// Close closes the underlying boltdb.
func (s *BoltMetaStore) Close() {
	s.db.Close()
}

// Objects returns all MetaObjects in the meta store
func (s *BoltMetaStore) Objects() ([]*MetaData, error) {
	var objects []*MetaData

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(objectsBucket)
		if bucket == nil {
			return errNoBucket
		}

		bucket.ForEach(func(k, v []byte) error {
			var meta MetaData
			dec := gob.NewDecoder(bytes.NewBuffer(v))
			err := dec.Decode(&meta)
			if err != nil {
				return err
			}
			objects = append(objects, &meta)
			return nil
		})
		return nil
	})

	return objects, err
}
