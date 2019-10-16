package awssh

import (
	"time"

	"github.com/adelowo/onecache"
	"github.com/adelowo/onecache/filesystem"
	"github.com/aws/aws-sdk-go/aws/credentials"
	homedir "github.com/mitchellh/go-homedir"
)

const (
	CachePath string = "~/.config/awssh/cache"
)

type Cache struct {
	Store   *filesystem.FSStore
	Marshal *onecache.CacheSerializer
	Key     string
}

func NewCache(path, key string) (c *Cache, err error) {
	fullPath, err := homedir.Expand(path)
	if err != nil {
		return nil, err
	}

	store := filesystem.MustNewFSStore(fullPath)
	marshal := onecache.NewCacheSerializer()

	c = &Cache{
		Store:   store,
		Marshal: marshal,
		Key:     key,
	}

	return c, nil
}

func (c *Cache) Save(creds *credentials.Value, expire time.Duration) (err error) {
	dataByte, err := c.Marshal.Serialize(&creds)
	if err != nil {
		return err
	}

	err = c.Store.Set(c.Key, dataByte, expire)
	return err
}

func (c *Cache) Load() (creds *credentials.Value, err error) {
	credsByte, err := c.Store.Get(c.Key)
	if err != nil {
		return nil, err
	}

	c.Marshal.DeSerialize(credsByte, &creds)

	return creds, nil
}

/*
func SaveCache(filePath, key string, creds *credentials.Value, expire time.Duration) (err error) {
	marshal := onecache.NewCacheSerializer()
	dataByte, err := marshal.Serialize(&creds)
	if err != nil {
		return err
	}

	fullPath, err := homedir.Expand(filePath)
	if err != nil {
		return err
	}

	store := filesystem.MustNewFSStore(fullPath)
	err = store.Set(key, dataByte, expire)
	return err
}

func LoadCache(filePath, key string) (creds *credentials.Value, err error) {
	fullPath, err := homedir.Expand(filePath)
	if err != nil {
		return nil, err
	}

	store := filesystem.MustNewFSStore(fullPath)
	credsByte, err := store.Get(key)
	if err != nil {
		return nil, err
	}

	marshal := onecache.NewCacheSerializer()
	marshal.DeSerialize(credsByte, &creds)

	return creds, nil
}
*/
