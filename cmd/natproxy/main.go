package main

import (
	"flag"
	"log"

	"github.com/jiajunhuang/natproxy/client"
	"github.com/jiajunhuang/natproxy/tools"
)

var (
	register   = flag.Bool("register", false, "是否注册")
	login      = flag.Bool("login", false, "是否登录")
	email      = flag.String("email", "", "注册邮箱")
	password   = flag.String("password", "", "注册密码")
	disconnect = flag.Bool("disconnect", false, "是否设置为断开连接")
	connect    = flag.Bool("connect", false, "是否设置为开启连接")
)

func main() {
	flag.Parse()

	if (*register || *login) && (*email == "" || *password == "") {
		log.Printf("邮箱和密码不能为空")
		return
	}

	if *register {
		if err := tools.Register(*email, *password); err != nil {
			log.Printf("注册失败: %s", err)
		} else {
			log.Printf("注册成功")
		}
		return
	}

	if *login {
		if token, err := tools.Login(*email, *password); err != nil {
			log.Printf("登录失败: %s", err)
		} else {
			log.Printf("登录成功，token是 %s", token)
		}
		return
	}

	client.Start(*connect, *disconnect)
}
