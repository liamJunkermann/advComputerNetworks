package main

import (
	"crypto/sha256"
	"errors"
	"hash"
	"sync"

	"github.com/golang/glog"
)

type DynamicBlock struct {
	Remoteaddr	string	 `json:"remoteAddr"`
	Method			string 	 `json:"method"`
	Url 				string   `json:"url"`
	Blocked 		bool		 `json:"blocked"`
}

type URLlist struct {
	hash 				hash.Hash
	UrlValues		map[string]DynamicBlock	`json:"urlValues"`
	busyValues	map[string]*sync.Mutex
	mutex				*sync.Mutex
}

func CreateList() (*URLlist, error) {
	url := make(map[string]DynamicBlock)
	busy := make(map[string]*sync.Mutex)

	hash := sha256.New()
	mutex := &sync.Mutex{}

	urlList := &URLlist{
		hash: hash,
		UrlValues: url,
		busyValues: busy,
		mutex: mutex,
	}

	return urlList, nil
}

func (u *URLlist) has(key string) (*sync.Mutex, bool) {
	hashValue := calcHash(key)

	u.mutex.Lock()
	defer u.mutex.Unlock()

	if lock, busy := u.busyValues[hashValue]; busy {
		u.mutex.Unlock()
		lock.Lock()
		lock.Unlock()
		u.mutex.Lock()
	}

	if _, found := u.UrlValues[hashValue]; found {
		return nil, true
	}

	lock := new(sync.Mutex)
	lock.Lock()
	u.busyValues[hashValue] = lock
	return lock, false
}

func (u *URLlist) get(key string) (*DynamicBlock, error) {
	hashValue := calcHash(key)

	u.mutex.Lock()
	url, ok := u.UrlValues[hashValue]
	u.mutex.Unlock()

	if !ok {
		glog.Info("URL Item", hashValue, " has not been logged before")
	}

	glog.Info("Found ", hashValue, " hash has been logged")
	return &url, nil
}

func (u *URLlist) block(hashValue string) (*DynamicBlock, error) {
	u.mutex.Lock()
	listing, ok := u.UrlValues[hashValue]
	u.mutex.Unlock()

	if !ok {
		glog.Error("URL Item", hashValue, " was not logged properly")
		return nil, errors.New("hash not found")
	}
	listing.Blocked = true

	glog.Info("Blocked hash: ", hashValue)

	defer u.release(hashValue, listing)
	return &listing, nil
}

func (u *URLlist) unblock(hashValue string) (*DynamicBlock, error) {
	u.mutex.Lock()
	listing, ok := u.UrlValues[hashValue]
	u.mutex.Unlock()

	if !ok {
		glog.Error("URL Item", hashValue, " was not logged properly")
		return nil, errors.New("hash not found")
	}
	listing.Blocked = false

	defer u.release(hashValue, listing)
	return &listing, nil
}

func (u *URLlist) put(key string, urlListing *DynamicBlock) error {
	hashValue := calcHash(key)
	glog.Info("putting ", hashValue, " with listing ", urlListing)
	defer u.release(hashValue, *urlListing)
	return nil
}

func (u *URLlist) release(hashValue string, urlValue DynamicBlock) {
	u.mutex.Lock()
	delete(u.busyValues, hashValue)
	u.UrlValues[hashValue] = urlValue
	u.mutex.Unlock()
}