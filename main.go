package main

import (
	"log"
	"net/rpc"
	"os"
	"os/signal"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("传参错误！")
	}

	nodeId := os.Args[1]
	node := NewNode(nodeId)

	listener, err := node.NewListener()
	if err != nil {
		log.Fatal("监听当前节点失败：", err.Error())
	}
	defer listener.Close()

	rpcServer := rpc.NewServer()
	rpcServer.Register(node)

	go rpcServer.Accept(listener)

	node.ConnectToSiblings()

	log.Printf("节点 %s 连接到 %s\n", nodeId, node.Siblings.ListSiblings())
	log.Println("等待系统启动……")

	time.Sleep(10 * time.Second)

	node.Elect()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<- ch
}