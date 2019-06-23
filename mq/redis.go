package mq

import (
	"time"

	"github.com/gomodule/redigo/redis"
)

type RedisDB struct {
	Pool *redis.Pool
}

var Redis *RedisDB

const (
	JOB_POOL_KEY    = "job_pool"
	BUCKET_KEY      = "bucket"
	READY_QUEUE_KEY = "ready_queue"
)

func init() {
	Redis = &RedisDB{
		Pool: &redis.Pool{
			MaxIdle:     30,
			MaxActive:   10000,
			IdleTimeout: 240 * time.Second,
			Wait:        true,
			Dial: func() (redis.Conn, error) {
				c, err := redis.Dial("tcp", "127.0.0.1:6379", redis.DialPassword(""))
				if err != nil {
					return nil, err
				}
				return c, nil
			},
			TestOnBorrow: func(c redis.Conn, t time.Time) error {
				if time.Since(t) < time.Minute {
					return nil
				}
				_, err := c.Do("PING")
				return err
			},
		},
	}
}

func (db *RedisDB) Do(command string, args ...interface{}) (interface{}, error) {
	conn := db.Pool.Get()
	defer conn.Close()
	return conn.Do(command, args...)
}

func (db *RedisDB) String(command string, args ...interface{}) (string, error) {
	return redis.String(db.Do(command, args...))
}

func (db *RedisDB) Bool(command string, args ...interface{}) (bool, error) {
	return redis.Bool(db.Do(command, args...))
}

func (db *RedisDB) Strings(command string, args ...interface{}) ([]string, error) {
	return redis.Strings(db.Do(command, args...))
}

func (db *RedisDB) Int(command string, args ...interface{}) (int, error) {
	return redis.Int(db.Do(command, args...))
}

func (db *RedisDB) Ints(command string, args ...interface{}) ([]int, error) {
	return redis.Ints(db.Do(command, args...))
}

func (db *RedisDB) StringMap(command string, args ...interface{}) (map[string]string, error) {
	return redis.StringMap(db.Do(command, args...))
}

func GetJobKeyById(id string) string {
	return JOB_POOL_KEY + ":" + id
}

func GetJobQueueByTopic(topic string) string {
	return READY_QUEUE_KEY + ":" + topic
}

func GetBucketKeyById(id string) string {
	return BUCKET_KEY + ":" + id
}
