package main

import (
	"log"
	"net"
	"net/rpc"
	"strings"
	"sync"
	"time"
)

// 各节点以及监听地址的映射信息
var nodeMap = map[string]string{
	"node1":    "127.0.0.1:8081",
	"node2":    "127.0.0.1:8082",
	"node3":    "127.0.0.1:8083",
	"node4":    "127.0.0.1:8084",
	"node5":    "127.0.0.1:8085",
}

// 节点信息
type Node struct {
	Id          string
	Addr        string
	LeaderId    string
	RpcClient   *rpc.Client
	Siblings    *Siblings
}

// 系统中其他节点的信息
type Siblings struct {
	Nodes       map[string]*Node
	*sync.RWMutex
}
/*********** Siblings method start ***********/
func newSiblings() *Siblings {
	siblings := &Siblings{
		Nodes:   make(map[string]*Node),
		RWMutex: &sync.RWMutex{},
	}

	return siblings
}

func (siblings *Siblings) addSibling(nodeId string, rpcClient *rpc.Client) {
	siblings.Lock()
	defer siblings.Unlock()

	siblings.Nodes[nodeId] = &Node{
		Id:        nodeId,
		RpcClient: rpcClient,
	}
}

func (siblings *Siblings) getSibling(nodeId string) *Node {
	siblings.Lock()
	defer siblings.Unlock()

	return siblings.Nodes[nodeId]
}

func (siblings *Siblings) deleteSibling(nodeId string) {
	siblings.Lock()
	defer siblings.Unlock()

	delete(siblings.Nodes, nodeId)
}

func (siblings *Siblings) ListSiblings() string {
	siblings.Lock()
	defer siblings.Unlock()

	nodeIds := make([]string, 0, len(siblings.Nodes))

	for nodeId := range siblings.Nodes {
		nodeIds = append(nodeIds, nodeId)
	}

	return strings.Join(nodeIds, ", ")
}

func (siblings *Siblings) sliceSiblings() []*Node {
	siblings.Lock()
	defer siblings.Unlock()

	nodes := make([]*Node, 0, len(siblings.Nodes))

	for _, node := range siblings.Nodes {
		nodes = append(nodes, node)
	}

	return nodes
}
/*********** Siblings method end ***********/

// 创建新的节点
func NewNode(id string) *Node {
	node := &Node{
		Id:        id,
		Addr:      nodeMap[id],
		RpcClient: nil,
		Siblings:  nil,
	}

	node.Siblings = newSiblings()

	return node
}

// 监听当前节点
func (node *Node) NewListener() (net.Listener, error) {
	listener, err := net.Listen("tcp", node.Addr)

	return listener, err
}

// 连接系统中其他节点
func (node *Node) ConnectToSiblings() {
	for nodeId, nodeAddr := range nodeMap {
		if nodeId == node.Id {
			// 跳过与自身的连接
			continue
		}

		rpcClient := node.connectToSibling(nodeAddr)
		// 检测节点通信是否正常
		pingMessage := Message{
			From: node.Id,
			Type: Ping,
		}
		response, _ := node.CommunicateWithSibling(rpcClient, pingMessage)

		if response.Type == Pong {
			log.Printf("与节点 %s 通信正常\n", nodeId)
			node.Siblings.addSibling(nodeId, rpcClient)
		} else {
			log.Printf("与节点 %s 通信异常，返回：%+v\n", nodeId, response)
		}
	}
}

// 连接到单个节点
func (node *Node) connectToSibling(nodeAddr string) *rpc.Client {
retry:
	rpcClient, err := rpc.Dial("tcp", nodeAddr)

	if err != nil {
		log.Printf("节点 %s 连接到节点 %s 失败: %v，即将进行重试……\n", node.Id, nodeAddr, err)
		time.Sleep(1 * time.Second)
		goto retry
	}

	return rpcClient
}

// 与节点进行通信
func (node *Node) CommunicateWithSibling(rpcClient *rpc.Client, message Message) (Message, error) {
	var response Message

	err := rpcClient.Call("Node.RespondTheMessage", message, &response)

	if err != nil {
		log.Printf("与节点通信失败：%s\n", err)
	}

	return response, err
}

// 响应消息
func (node *Node) RespondTheMessage(message Message, response *Message) error {
	response.From = node.Id

	switch message.Type {
	case Ping:
		response.Type = Pong
		// 如果之前的 leader 节点重启之后重新与系统中其他节点连接，那么当前的节点也需要与这个重启的节点建立连接
		if node.Siblings.getSibling(message.From) == nil {
			log.Printf("节点 %s 重新与节点 %s 建立连接\n", node.Id, message.From)
			rpcClient := node.connectToSibling(nodeMap[message.From])
			node.Siblings.addSibling(message.From, rpcClient)
		}
	case Election:
		response.Type = Alive
	case Elected:
		node.LeaderId = message.From
		log.Printf("leader 节点选举完成，新的 leader 节点为 %s\n", node.LeaderId)
		// 对新的 leader 进行心跳检测
		go node.heartBeat()
		response.Type = Alive
	}

	return nil
}

// leader 节点心跳检测
func (node *Node) heartBeat() {
ping:
	leader := node.Siblings.getSibling(node.LeaderId)

	if leader == nil {
		log.Printf("获取当前 leader 节点失败，nodeId: %s， leaderId: %s， siblings: %s\n",
			node.Id, node.LeaderId, node.Siblings.ListSiblings())
		return
	}

	message := Message{
		From: node.Id,
		Type: Ping,
	}

	response, err := node.CommunicateWithSibling(leader.RpcClient, message)

	if err != nil {
		log.Printf("当前 leader 节点失效，开始选举新的 leader 节点\n")
		node.Siblings.deleteSibling(node.LeaderId)
		node.LeaderId = ""
		// 选举新的 leader 节点
		node.Elect()
		return
	}

	log.Printf("leader 节点心跳检测响应：%v\n", response)
	if response.Type == Pong {
		time.Sleep(5 * time.Second)
		goto ping
	}
}

// 选举 leader 节点
func (node *Node) Elect() {
	isHighestNode := true

	siblings := node.Siblings.sliceSiblings()

	for _, sibling := range siblings {
		if strings.Compare(node.Id, sibling.Id) > 0 {
			continue
		}

		log.Printf("%s 发送 election 消息到 %s\n", node.Id, sibling.Id)
		message := Message{
			From: node.Id,
			Type: Election,
		}

		response, _ := node.CommunicateWithSibling(sibling.RpcClient, message)

		if response.Type == Alive {
			isHighestNode = false
		}
	}

	if isHighestNode {
		node.LeaderId = node.Id
		electedMessage := Message{
			From: node.Id,
			Type: Elected,
		}
		// 广播消息
		node.broadCast(electedMessage)
	}
}

// 广播消息
func (node *Node) broadCast(message Message) {
	siblings := node.Siblings.sliceSiblings()

	for _, sibling := range siblings {
		log.Printf("节点 %s 广播 elected 消息到 %s\n", node.Id, sibling.Id)
		response, err := node.CommunicateWithSibling(sibling.RpcClient, message)
		if err != nil {
			log.Printf("广播 elected 消息到 %s 失败：%s\n", sibling.Id, err)
		} else {
			log.Printf("广播 elected 消息到 %s 得到响应：%v\n", sibling.Id, response)
		}
	}
}