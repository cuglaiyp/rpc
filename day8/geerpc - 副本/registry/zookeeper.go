package registry

import (
	"fmt"
	"github.com/go-zookeeper/zk"
	"log"
	"strings"
	"time"
)

type ZkClient struct {
	conn *zk.Conn
}

func NewZkClient(addr string, timeout time.Duration) *ZkClient {
	if timeout == 0 {
		//  没给超时时间的话，就以默认的超时间发一次，但是不能卡着点发，因为发送还需要时间
		timeout = defaultTimeout - time.Duration(1) * time.Minute
	}
	conn, _, err := zk.Connect([]string{addr}, timeout)
	if err != nil {
		log.Printf("rpc registry: cannot connect to zookeeper: %s: err: %s", addr, err.Error())
		return nil
	}
	return &ZkClient{conn: conn}

}

const (
	root          = "/geerpc"    // zookeeper 的根结点
	providers     = "/providers" // 二级结点
)

func (c *ZkClient) PutServer(server string) {
	c.Put(root+providers+"/"+server, nil, zk.FlagEphemeral)
}

func (c *ZkClient) Put(path string, data []byte, nodeType int32) error {
	exists, _, err := c.conn.Exists(path)

	if err != nil {
		return err
	}
	if !exists {
		err = c.ensureRoot(path)
		if err != nil {
			return err
		}
		_, err = c.conn.Create(path, data, nodeType, zk.WorldACL(zk.PermAll))
		if err != nil && err != zk.ErrNodeExists {
			return err
		}
		return nil
	}
	// 存在更新一下
	bytes, stat, err := c.conn.Get(path)
	if err != nil {
		return err
	}
	if string(bytes[:]) == string(data[:]) {
		return nil
	}
	_, err = c.conn.Set(path, data, stat.Version)
	return err
}

func (c *ZkClient) ensureRoot(path string) error {
	rootPath, err := c.getRoot(path)
	if err != nil {
		return err
	}
	if rootPath == "" {
		return nil
	}
	return c.Put(rootPath, nil, 0)
}

func (c *ZkClient) getRoot(path string) (string, error) {
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return "", fmt.Errorf("Error path: %s", path)
	}
	return path[0:idx], nil
}

