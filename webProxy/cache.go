package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/golang/glog"
)

type Cache struct {
	folder       string
	hash         hash.Hash
	knownValues  map[string][]byte
	timingValues map[string]time.Duration
	busyValues   map[string]*sync.Mutex
	mutex        *sync.Mutex
}

func CreateCache(path string) (*Cache, error) {
	fileInfos, err := ioutil.ReadDir(path)
	if err != nil {
		glog.Error("Cannot open cache folder ", path, ": ", err)
		glog.Info("Create cache folder ", path)
		os.Mkdir(path, os.ModePerm)
	}

	values := make(map[string][]byte)
	timeValues := make(map[string]time.Duration)
	busy := make(map[string]*sync.Mutex)

	// Go through every file an save its name in the map. The content of the file
	// is loaded when needed. This makes sure that we don't have to read
	// the directory content each time the user wants data that's not yet loaded.
	for _, info := range fileInfos {
		if !info.IsDir() {
			values[info.Name()] = nil
		}
	}

	hash := sha256.New()

	mutex := &sync.Mutex{}

	cache := &Cache{
		folder:      path,
		hash:        hash,
		knownValues: values,
		timingValues: timeValues,
		busyValues:  busy,
		mutex:       mutex,
	}

	return cache, nil
}

// Returns true if the resource is found, and false otherwise. If the
// resource is busy, this method will hang until the resource is free. If
// the resource is not found, a lock indicating that the resource is busy will
// be returned. Once the resource has been put into cache the busy lock *must*
// be unlocked to allow others to access the newly cached resource
func (c *Cache) has(key string) (*sync.Mutex, bool) {
	hashValue := calcHash(key)

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// If the resource is busy, wait for it to be free. This is the case if
	// the resource is currently being cached as a result of another request.
	// Also, release the lock on the cache to allow other readers while waiting
	if lock, busy := c.busyValues[hashValue]; busy {
		c.mutex.Unlock()
		lock.Lock()
		lock.Unlock()
		c.mutex.Lock()
	}

	// If a resource is in the shared cache, it can't be reserved. One can simply
	// access it directly from the cache
	if _, found := c.knownValues[hashValue]; found {
		return nil, true
	}

	// The resource is not in the cache, mark the resource as busy until it has
	// been cached successfully. Unlocking lock is required!
	lock := new(sync.Mutex)
	lock.Lock()
	c.busyValues[hashValue] = lock
	return lock, false
}

func (c *Cache) get(key string) (*io.Reader, error) {
	var response io.Reader
	hashValue := calcHash(key)

	// Try to get content. Error if not found.
	c.mutex.Lock()
	content, ok := c.knownValues[hashValue]
	timing := c.timingValues[hashValue]
	c.mutex.Unlock()
	if !ok && len(content) > 0 {
		glog.Info("Cache doesn't know key ", hashValue)
		return nil, fmt.Errorf("key '%s' is not known to cache", hashValue)
	}

	glog.Info("Cache has key", hashValue)

	// Key is known, but not loaded into RAM
	if content == nil {
		glog.Info("Cache item ", hashValue, " known but is not stored in memory. Using file.")

		file, err := os.Open(c.folder + hashValue)
		if err != nil {
			glog.Error("Error reading cached file ", hashValue, ": ", err)
			return nil, err
		}

		response = file

		glog.Info("Create reader from file ", hashValue)
	} else { // Key is known and data is already loaded to RAM
		response = bytes.NewReader(content)
		glog.Info("Create reader from ", len(content), " byte large cache content")
	}

	glog.Info("Saved ", timing.Milliseconds(), "ms and ", len(content), "bytes")
	return &response, nil
}

// release is an internal method which atomically caches an item and unmarks
// the item as busy, if it was busy before. The busy lock *must* be unlocked
// elsewhere!
func (c *Cache) release(hashValue string, content []byte, timing time.Duration) {
	c.mutex.Lock()
	delete(c.busyValues, hashValue)
	c.knownValues[hashValue] = content
	c.timingValues[hashValue] = timing
	c.mutex.Unlock()
}

func (c *Cache) put(key string, content *io.Reader, contentLength int64, timing time.Duration) error {
	hashValue := calcHash(key)

	// Small enough to put it into the in-memory cache
	if contentLength <= config.MaxCacheItemSize*1024*1024 {
		buffer := &bytes.Buffer{}
		_, err := io.Copy(buffer, *content)
		if err != nil {
			return err
		}


		defer c.release(hashValue, buffer.Bytes(), timing)
		glog.Info("Added ", hashValue, "into in-memory cache")

		err = ioutil.WriteFile(c.folder+hashValue, buffer.Bytes(), 0644)
		if err != nil {
			return err
		}
		glog.Info("Wrote content of entry ", hashValue, " into file")
	} else { // Too large for in-memory cache, just write to file
		defer c.release(hashValue, nil, time.Since(time.Now()))
		glog.Info("Added nil-entry for ", hashValue, " into in-memory cache")

		file, err := os.Create(c.folder + hashValue)
		if err != nil {
			return err
		}

		writer := bufio.NewWriter(file)
		_, err = io.Copy(writer, *content)
		if err != nil {
			return err
		}
		glog.Info("Wrote content of entry ", hashValue, " into file")
	}

	glog.Info("Cache wrote content into ", hashValue)

	return nil
}

func calcHash(data string) string {
	sha := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sha[:])
}
