# 简易 rpc 框架 :rocket:
仿照Go 语言官方的标准库 net/rpc，进行开发，并在此基础上，新增了协议交换、注册中心、服务发现、负载均衡、超时处理等特性。
### 主要特点：
- :hammer: 编解码部分除了实现了 Json、Gob 格式，还实现了自定义的 TLV 编码
- :dart: 负载均衡采用了客户端负载均衡策略，实现了随机、轮询和一致性 Hash 三种算法
- :cloud: 实现了简易的注册中心和心跳机制，同时支持了 Zookeeper 和 Etcd 
- :clock10: 使用 time.After 和 select-chan 机制为客户端连接、服务端处理添加了超时处理机制
