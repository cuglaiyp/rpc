package geerpc

import (
	"fmt"
	"net/http"
	"text/template"
)

// 因为 Client 和 Server 都支持了 Http 服务，那我们为这个特性添加一个扩展功能：在 debug 页面上展示服务的调用统计视图。

// 首先定义一个 html 模板字符串
const debugText = `<html>
	<body>
	<title>GeeRPC Services</title>
	{{range .}}
	<hr>
	Service {{.Name}}
	<hr>
		<table>
		<th align=center>Method</th><th align=center>Calls</th>
		{{range $name, $mtype := .Method}}
			<tr>
			<td align=left font=fixed>{{$name}}({{$mtype.ArgType}}, {{$mtype.ReplyType}}) error</td>
			<td align=center>{{$mtype.NumCalls}}</td>
			</tr>
		{{end}}
		</table>
	{{end}}
	</body>
	</html>`

// Must 就是用来把 Parse 方法的两个返回值变成一个，这样的用函数来定义全局变量就比较方便
var debug = template.Must(template.New("RPC debug").Parse(debugText))

// 定义该功能的一些结构体对象

type debugHTTP struct {
	*Server // 继承 *Server
}

// 为模板解析准备一个结构体
type debugService struct {
	Name string
	Method map[string]*methodType
}

// 重写该方法
func (d debugHTTP) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var services []debugService
	d.serviceMap.Range(func(key, value interface{}) bool {
		svc := value.(*service)
		services = append(services, debugService{
			Name:   key.(string),
			Method: svc.methods,
		})
		return true
	})
	// 解析模板，并写回响应
	err := debug.Execute(w, services)
	if err != nil {
		fmt.Fprintf(w, "rpc: error executing template: %s", err.Error())
	}
}