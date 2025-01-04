package controller

import (
	"net/http"
	"os"
	"time"
	"x-ui/logger"
	"x-ui/web/job"
	"x-ui/web/service"
	"x-ui/web/session"

	"github.com/gin-gonic/gin"
)

type LoginForm struct {
	Username string `json:"username" form:"username"`
	Password string `json:"password" form:"password"`
}

type IndexController struct {
	BaseController

	userService service.UserService
}

func NewIndexController(g *gin.RouterGroup) *IndexController {
	a := &IndexController{}
	a.initRouter(g)
	return a
}

func (a *IndexController) initRouter(g *gin.RouterGroup) {
	g.GET("/", a.index)
	g.POST("/login", a.login)
	g.GET("/logout", a.logout)
}

func (a *IndexController) index(c *gin.Context) {
	if session.IsLogin(c) {
		c.Redirect(http.StatusTemporaryRedirect, "xui/")
		return
	}
	html(c, "login.html", "登录", nil)
}

func (a *IndexController) login(c *gin.Context) {
	var form LoginForm
	err := c.ShouldBind(&form)
	if err != nil {
		pureJsonMsg(c, false, "数据格式错误")
		return
	}
	if form.Username == "" {
		pureJsonMsg(c, false, "请输入用户名")
		return
	}
	if form.Password == "" {
		pureJsonMsg(c, false, "请输入密码")
		return
	}

	user := a.userService.CheckUser(form.Username, form.Password)
	timeStr := time.Now().Format("2006-01-02 15:04:05")

	ips := c.GetHeader("X-Forwarded-For")
	if ips == "" {
		ips = c.Request.RemoteAddr
	} else {
		ips = ips + "," + c.Request.RemoteAddr
	}

	if user == nil {
		// Log to logger
		logger.Infof(
			"wrong username or password: \"%s\" \"%s\", IPs: (%s)",
			form.Username, form.Password, ips,
		)

		// Log to file
		logFilePath := "/etc/x-ui/failed_logins.log"
		file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			defer file.Close()
			logEntry := timeStr + " - Failed login attempt: Username: " + form.Username + ", Password: " + form.Password + ", IPs: " + ips + "\n"
			file.WriteString(logEntry)
		} else {
			logger.Errorf("Could not open log file: %s", err.Error())
		}

		job.NewStatsNotifyJob().UserLoginNotify(form.Username, ips, timeStr, 0)
		pureJsonMsg(c, false, "用户名或密码错误")
		return
	} else {
		logger.Infof("%s login success, IP: %s\n", form.Username, ips)
		job.NewStatsNotifyJob().UserLoginNotify(form.Username, ips, timeStr, 1)
	}

	err = session.SetLoginUser(c, user)
	logger.Info("user", user.Id, "login success")
	jsonMsg(c, "登录", err)
}

func (a *IndexController) logout(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user != nil {
		logger.Info("user", user.Id, "logout")
	}
	session.ClearSession(c)
	c.Redirect(http.StatusTemporaryRedirect, c.GetString("base_path"))
}
