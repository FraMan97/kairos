package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/FraMan97/kairos/client/internal/config"
	"github.com/boltdb/bolt"
)

func OpenDatabase() (*bolt.DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dbDir := filepath.Join(home, ".kairos", "client", "database")
	err = os.MkdirAll(dbDir, 0700)
	if err != nil {
		return nil, err
	}
	db, err := bolt.Open(filepath.Join(dbDir, "kairos_boltdb.db"), 0600, &bolt.Options{})
	if err != nil {
		return nil, err
	}
	log.Printf("[%s] - BoltDB opened in '%s'\n", config.DatabaseService, dbDir)
	config.BoltDB = db
	return db, err
}

func EnsureBucket(db *bolt.DB, bucketName string) error {
	return db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return err
		}
		log.Printf("[%s] - Bucket '%s' found in DB\n", config.DatabaseService, bucketName)
		return nil
	})
}

func GetData(db *bolt.DB, bucketName string, key string) ([]byte, error) {
	var value []byte

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("[%s] - bucket '%s' not found", config.DatabaseService, bucketName)
		}

		v := b.Get([]byte(key))
		if v == nil {
			return fmt.Errorf("[%s] - key '%s' not found in bucket '%s'", config.DatabaseService, key, bucketName)
		}

		value = make([]byte, len(v))
		copy(value, v)

		log.Printf("[%s] - Data found in bucket '%s' with key '%s'", config.DatabaseService, bucketName, key)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return value, nil
}

func PutData(db *bolt.DB, bucketName string, key string, data []byte) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("[%s] - bucket '%s' not found", config.DatabaseService, bucketName)
		}
		err := b.Put([]byte(key), data)
		if err != nil {
			return err
		}
		log.Printf("[%s] - Inserted in bucket '%s' new data with key '%s'\n", config.DatabaseService, bucketName, key)
		return nil
	})
}

func DeleteKey(db *bolt.DB, bucketName string, key string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("[%s] - bucket '%s' not found", config.DatabaseService, bucketName)
		}
		err := b.Delete([]byte(key))
		if err != nil {
			return err
		}
		log.Printf("[%s] - Deleted the key '%s' in bucket '%s'\n", config.DatabaseService, key, bucketName)
		return nil
	})
}

func ExistsKey(db *bolt.DB, bucketName string, key string) (bool, error) {
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("bucket '%s' not found in DB", bucketName)
		}
		v := b.Get([]byte(key))
		if v == nil {
			return fmt.Errorf("key '%s' not found in bucket %s", bucketName, key)
		}
		log.Printf("[%s] - Key '%s' exists in the bucket %s", config.DatabaseService, key, bucketName)
		return nil
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func GetAllData(db *bolt.DB, bucketName string) (map[string][]byte, error) {
	values := make(map[string][]byte)

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("[%s] - bucket '%s' not found in DB", config.DatabaseService, bucketName)
		}

		return b.ForEach(func(k []byte, v []byte) error {
			valueCopy := make([]byte, len(v))
			copy(valueCopy, v)

			values[string(k)] = valueCopy
			return nil
		})

	})

	if err != nil {
		return nil, err
	}

	log.Printf("[%s] - All data from the bucket '%s' are extracted successfully from DB", config.DatabaseService, bucketName)
	return values, nil
}

func GetAllKeys(db *bolt.DB, bucketName string) ([]string, error) {
	values := []string{}

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("bucket '%s' not found", bucketName)
		}

		return b.ForEach(func(k []byte, v []byte) error {
			values = append(values, string(k))
			return nil
		})

	})

	if err != nil {
		return nil, err
	}

	log.Printf("[%s] - All keys from the bucket '%s' are extracted successfully from DB", config.DatabaseService, bucketName)
	return values, nil
}
