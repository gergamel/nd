package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"net/http"
)

var (
	errHashMismatch = errors.New("Content hash does not match OID")
	errSizeMismatch = errors.New("Content size does not match")
)

// FsObjectStore implements simple file-per object binary storage within
// a filesystem folder
type FsObjectStore struct {
	path string
}

// NewContentStore creates a ContentStore at the base directory.
func NewFsObjectStore(path string) (*FsObjectStore, error) {
	err := os.MkdirAll(path, 0750)
	if err != nil {
		return nil, err
	}
	return &FsObjectStore{path}, nil
}

// List returns an array of hash strings for every object in the store.
// TODO: Move this over to the metastore and use paging queries
//       or this will be a weak point once there are a lot of files.
func (s *FsObjectStore) List() ([]string, error) {
	files, err := ioutil.ReadDir(s.path)
	if err != nil {
		return nil, err
	}
	
	N := len(files)
	result := make([]string, N)
	for i := 0; i < N; i++ {
		result[i] = files[i].Name()
	}
	
	return result, nil
}

// Get takes an hash string and and retreives the content from the store, returning
// it as an io.ReaderCloser. If fromByte > 0, the reader starts from that byte
func (s *FsObjectStore) Get(hash string, fromByte int64) (io.ReadCloser, error) {
	path := filepath.Join(s.path, hash)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if fromByte > 0 {
		_, err = f.Seek(fromByte, os.SEEK_CUR)
	}
	return f, err
}

// DetectContentType takes a hash and attempts to determine the
// MIME type of the associated object using net/http.DetectContentType
// Returns "application/octet-stream" in the event of any errors.
func (s *FsObjectStore) DetectContentType(hash string) string {
	path := filepath.Join(s.path, hash)
	f, err := os.Open(path)
	if err != nil {
		return "application/octet-stream"
	}
	defer f.Close()
	
	// Only the first 512 bytes are used to sniff the content type.
	b := make([]byte, 512)
	_, err = f.Read(b)
	if err != nil {
		return "application/octet-stream"
	}
	contentType := http.DetectContentType(b)
	return contentType
}

/*
 * Put takes an expected hash value and a io.Reader and attempts to store
 * it into the content store. Write initially happens into a <hash>.tmp
 * file and, upon completion:
 * 1) If the calculated hash matches the expected, the <hash>.tmp file
 *    is renamed to <hash> and the error is nil.
 * 2) If the hash doesn't match, <hash>.tmp is deleted and the returned
 *    error is errHashMismatch.
 */
func (s *FsObjectStore) Put(hash string, r io.Reader) (int64, error) {
	path := filepath.Join(s.path, hash)
	tmpPath := path + ".tmp"

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return 0, err
	}
	
	// Create the .tmp file
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0640)
	if err != nil {
		return 0, err
	}
	defer os.Remove(tmpPath)
	
	// Write to the .tmp file and calculate the sha256 at the same time
	h := sha256.New()
	hw := io.MultiWriter(h, file)
	written, err := io.Copy(hw, r)
	if err != nil {
		file.Close()
		return 0, err
	}
	logger.Log(kv{"method": "FsObjectStore.Put()", "hash": hash, "length": written})
	file.Close()
	
	// Chech the hash matches or error out
	hash_chk := hex.EncodeToString(h.Sum(nil))
	if hash_chk != hash {
		logger.Log(kv{"method": "FsObjectStore.Put()", "hash": hash, "calulated_hash": hash_chk})
		return 0, errHashMismatch
	}
	
	// Rename the file to drop the .tmp
	if err := os.Rename(tmpPath, path); err != nil {
		return 0, err
	}
	
	return written, nil
}

/*
 * Exists returns true if the object exists in the content store.
 * TODO: Other implementations (like rolling hash dedup store) will
 * likely involve a request to create a new object and, at the same
 * time, claim some kind of lock on that object. The implementation here
 * is a bit simplistic, but should still be okay for now because it uses
 * the sha256 hash as the file name. This means we shouldn't hit any
 * kind of race or collision because you'll almost never get 2 people
 * uploading the same file at the same time.
 */
func (s *FsObjectStore) Exists(hash string) bool {
	path := filepath.Join(s.path, hash)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}
