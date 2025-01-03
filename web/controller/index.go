package controller

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
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
	ip := getRemoteIp(c)

	if user == nil {
		// Handle failed login
		incrementFailedLoginCount(ip)

		// Check how many failed attempts so far
		attempts := getFailedLoginCount(ip)
		logger.Infof(
			"wrong username or password: \"%s\" \"%s\", IP: %s (Failed Attempts: %d)",
			form.Username, form.Password, ip, attempts,
		)

		// If attempts exceed 5, block the IP
		if attempts > 5 {
			blockIP(ip)
			logger.Infof("Blocked IP: %s due to repeated failed login attempts", ip)
		}

		job.NewStatsNotifyJob().UserLoginNotify(form.Username, ip, timeStr, 0)
		pureJsonMsg(c, false, "用户名或密码错误")
		return
	} else {
		// Reset the failed attempts on successful login (optional)
		resetFailedLoginCount(ip)

		logger.Infof("%s login success, IP: %s\n", form.Username, ip)
		job.NewStatsNotifyJob().UserLoginNotify(form.Username, ip, timeStr, 1)
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

// -----------------------------------------------------------------------------
// Utility functions for handling failed login attempts in /etc/x-ui/failed_logins.csv
// -----------------------------------------------------------------------------

// getFailedLoginData reads /etc/x-ui/failed_logins.csv into a map[ip] = count.
func getFailedLoginData() map[string]int {
	data := make(map[string]int)

	file, err := os.Open("/etc/x-ui/failed_logins.csv")
	if err != nil {
		// If the file doesn't exist or can't open, return empty map
		return data
	}
	defer file.Close()

	csvReader := csv.NewReader(file)
	records, err := csvReader.ReadAll()
	if err != nil {
		return data
	}

	for _, record := range records {
		if len(record) < 2 {
			continue
		}
		ip := record[0]
		count, err := strconv.Atoi(record[1])
		if err != nil {
			continue
		}
		data[ip] = count
	}
	return data
}

// saveFailedLoginData writes the map[ip] = count into /etc/x-ui/failed_logins.csv.
func saveFailedLoginData(data map[string]int) {
	file, err := os.Create("/etc/x-ui/failed_logins.csv")
	if err != nil {
		logger.Error("Could not create /etc/x-ui/failed_logins.csv:", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for ip, count := range data {
		record := []string{ip, strconv.Itoa(count)}
		if err := writer.Write(record); err != nil {
			logger.Error("Error writing record to CSV:", err)
		}
	}
}

// getFailedLoginCount returns the current failed login count for a given IP.
func getFailedLoginCount(ip string) int {
	data := getFailedLoginData()
	return data[ip]
}

// incrementFailedLoginCount increments the failed attempts for a given IP.
func incrementFailedLoginCount(ip string) {
	data := getFailedLoginData()
	data[ip] = data[ip] + 1
	saveFailedLoginData(data)
}

// resetFailedLoginCount sets the failed attempts for an IP to 0 (on successful login, if desired).
func resetFailedLoginCount(ip string) {
	data := getFailedLoginData()
	if _, ok := data[ip]; ok {
		data[ip] = 0
		saveFailedLoginData(data)
	}
}

// blockIP executes an iptables command to block the given IP address.
func blockIP(ip string) {
	cmd := exec.Command("iptables", "-I", "INPUT", "-s", ip, "-j", "DROP")
	err := cmd.Run()
	if err != nil {
		logger.Errorf("Error blocking IP %s: %v", ip, err)
	} else {
		logger.Infof("Successfully blocked IP %s with iptables", ip)
	}
}
