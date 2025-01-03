package controller

import (
	"encoding/csv"
	"net/http"
	"os"
	"strconv"
	"time"
	"x-ui/logger"
	"x-ui/web/job"
	"x-ui/web/service"
	"x-ui/web/session"

	"github.com/gin-gonic/gin"
	"sync"
)

var (
	failedLoginAttempts = make(map[string]int)
	mutex               sync.Mutex
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

// logFailedLogin logs the failed login attempt to a CSV file
func logFailedLogin(ip string) {
	mutex.Lock()
	defer mutex.Unlock()

	// Increment the counter for the given IP
	failedLoginAttempts[ip]++

	// Open the CSV file
	filePath := "/etc/x-ui/failed_logins.csv"
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		logger.Errorf("failed to open file %s: %v", filePath, err)
		return
	}
	defer file.Close()

	// Write to the CSV file
	writer := csv.NewWriter(file)
	defer writer.Flush()

	for ip, attempts := range failedLoginAttempts {
		record := []string{ip, strconv.Itoa(attempts)}
		if err := writer.Write(record); err != nil {
			logger.Errorf("failed to write to file %s: %v", filePath, err)
			return
		}
	}
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
	if user == nil {
		ip := getRemoteIp(c)
		job.NewStatsNotifyJob().UserLoginNotify(form.Username, ip, timeStr, 0)
		logger.Infof("wrong username or password: \"%s\" \"%s\", IP: %s", form.Username, form.Password, ip)
		pureJsonMsg(c, false, "用户名或密码错误")
		logFailedLogin(ip)
		return
	} else {
		logger.Infof("%s login success,Ip Address:%s\n", form.Username, getRemoteIp(c))
		job.NewStatsNotifyJob().UserLoginNotify(form.Username, getRemoteIp(c), timeStr, 1)
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
