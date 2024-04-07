# bully-algo
&emsp;&emsp;用 Golang 实现的分布式系统 leader 节点选举算法 Bully

&emsp;&emsp;在 Bully 算法中，每个节点都会被分配一个 ID，ID 最大的节点会被选举为系统当前的 leader 节点。如果当前 leader 节点失效，则系统会在当前仍然活跃的节点中选举 ID 最大的为新的 leader 节点。另外，当原先失效的节点从故障中恢复，也会触发新一轮的 leader 节点选举过程。

```go
var nodeMap = map[string]string{
	"node1":    "127.0.0.1:8081",
	"node2":    "127.0.0.1:8082",
	"node3":    "127.0.0.1:8083",
	"node4":    "127.0.0.1:8084",
	"node5":    "127.0.0.1:8085",
}
```

&emsp;&emsp;通过监听同一台机器上的不同端口，模拟了由 5 个节点组成的分布式系统的 leader 节点选举过程。在每次运行程序时都需要指定节点 ID：

```shell
go run bully-algo node1
go run bully-algo node2
go run bully-algo node3
go run bully-algo node4
go run bully-algo node5
```
