package controller

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"strconv"
	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/web/global"
	"x-ui/web/service"
	"x-ui/web/session"
)

type InboundController struct {
	inboundService service.InboundService
	xrayService    service.XrayService
}

func NewInboundController(g *gin.RouterGroup) *InboundController {
	a := &InboundController{}
	a.initRouter(g)
	a.startTask()
	return a
}

func (a *InboundController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/inbound")

	g.POST("/list", a.getInbounds)
	g.POST("/paged-list", a.getPagedInbounds)
	g.POST("/add", a.addInbound)
	g.POST("/del/:id", a.delInbound)
	g.POST("/update/:id", a.updateInbound)
}

func (a *InboundController) startTask() {
	webServer := global.GetWebServer()
	c := webServer.GetCron()
	c.AddFunc("@every 10s", func() {
		if a.xrayService.IsNeedRestartAndSetFalse() {
			err := a.xrayService.RestartXray(false)
			if err != nil {
				logger.Error("restart xray failed:", err)
			}
		}
	})
}

func (a *InboundController) getInbounds(c *gin.Context) {
	user := session.GetLoginUser(c)
	inbounds, err := a.inboundService.GetInbounds(user.Id)
	if err != nil {
		jsonMsg(c, "获取", err)
		return
	}
	jsonObj(c, inbounds, nil)
}

func (a *InboundController) getPagedInbounds(c *gin.Context) {
	// Get user and pagination parameters
	user := session.GetLoginUser(c)

	page, err := strconv.Atoi(c.DefaultPostForm("page", "1")) // Default to page 1
	if err != nil || page < 1 {
		page = 1
	}

	perpage, err := strconv.Atoi(c.DefaultPostForm("perpage", "10")) // Default to 10 per page
	if err != nil || perpage < 1 {
		perpage = 10
	}

	// query
	//query := c.DefaultPostForm("query", "")

	inbounds, totalCount, totalDown, totalUp, err := a.inboundService.GetPagedInbounds(user.Id, page, perpage, "")
	if err != nil {
		jsonMsg(c, "获取", err)
		return
	}

	// Calculate total pages
	totalPages := (totalCount + int64(perpage) - 1) / int64(perpage) // Round up division

	// Prepare response
	response := gin.H{
		"inbounds":     inbounds,
		"total_count":  totalCount,
		"total_pages":  totalPages,
		"total_down":   totalDown,
		"total_up":     totalUp,
		"current_page": page,
		"per_page":     perpage,
	}

	jsonObj(c, response, nil)
}

func (a *InboundController) addInbound(c *gin.Context) {
	inbound := &model.Inbound{}
	err := c.ShouldBind(inbound)
	if err != nil {
		jsonMsg(c, "添加", err)
		return
	}
	user := session.GetLoginUser(c)
	inbound.UserId = user.Id
	inbound.Enable = true
	inbound.Tag = fmt.Sprintf("inbound-%v", inbound.Port)
	err = a.inboundService.AddInbound(inbound)
	if err == nil {
		a.xrayService.SetToNeedRestart()
		//	GetInboundByPort
		inboundResult, err := a.inboundService.GetInboundByPort(inbound.Port)
		if err != nil {
			jsonMsg(c, "添加", err)
			return
		}
		jsonObj(c, inboundResult, nil)
	} else {
		jsonMsg(c, "添加", err)
	}
}

func (a *InboundController) delInbound(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "删除", err)
		return
	}
	err = a.inboundService.DelInbound(id)
	jsonMsg(c, "删除", err)
	if err == nil {
		a.xrayService.SetToNeedRestart()
	}
}

func (a *InboundController) updateInbound(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "修改", err)
		return
	}
	inbound := &model.Inbound{
		Id: id,
	}
	err = c.ShouldBind(inbound)
	if err != nil {
		jsonMsg(c, "修改", err)
		return
	}
	err = a.inboundService.UpdateInbound(inbound)
	if err == nil {
		a.xrayService.SetToNeedRestart()
		inboundResult, err := a.inboundService.GetInbound(id)
		if err != nil {
			jsonMsg(c, "修改", err)
			return
		}
		jsonObj(c, inboundResult, nil)
	} else {
		jsonMsg(c, "修改", err)
	}
}
