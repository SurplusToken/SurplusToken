package handler

import (
	"fmt"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// CarpoolWebSessionHandler 暴露拼车「打开网页版 ChatGPT」会话票据签发。
// 权限与可见性在 service 层收口（与 upstream-quota 同一道闸），这里只做参数解析与响应。
type CarpoolWebSessionHandler struct {
	service *service.CarpoolWebSessionService
}

func NewCarpoolWebSessionHandler(webSessionService *service.CarpoolWebSessionService) *CarpoolWebSessionHandler {
	return &CarpoolWebSessionHandler{service: webSessionService}
}

// Create handles POST /carpools/:id/web-session（幂等供给 Access + 签发 ES256 ticket）。
func (h *CarpoolWebSessionHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	result, err := h.service.Create(c.Request.Context(), id, subject.UserID, isCarpoolAdmin(c))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"web_session": result})
}

// Launch handles GET /carpools/:id/web-session/launch —— 「打开网页版 ChatGPT」
// 按钮 window.open 直开的自动兑换页。注册为免鉴权路由（弹窗导航带不了
// Authorization 头），但页面不含敏感数据：ticket 仍需页面脚本带同源
// localStorage 的 Bearer token 调 Create 才拿得到。
//
// 为什么必须是服务端路由而不是前端 doc.write 进弹窗：站点全局 CSP 禁止
// inline script 且 form-action 限 'self'，about:blank 弹窗继承 opener 的 CSP，
// 写进去的兑换脚本/跨源表单会被静默拦掉（弹窗停在「正在打开…」、网关收不到
// POST）。本页自带放行的 CSP（仅允许自己 inline script + 仅指向网关的
// form-action），脚本换取 ticket 后原地提交表单到网关 redeem——classic
// SAML POST-binding 模式，Safari/Chrome 通吃。
func (h *CarpoolWebSessionHandler) Launch(c *gin.Context) {
	id, ok := parseCarpoolID(c)
	if !ok {
		return
	}
	// form-action 只放行 desktop2web 网关 origin；功能未启用时 'none'。
	formAction := "'none'"
	if origin := h.service.PublicOrigin(); origin != "" {
		formAction = origin
	}
	c.Header("Content-Security-Policy",
		"default-src 'none'; script-src 'unsafe-inline'; connect-src 'self'; form-action "+formAction+"; base-uri 'none'")
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(webSessionLaunchPage(id)))
}

// webSessionLaunchPage 是 launch 端点返回的自动兑换页。内联脚本：取
// auth_token → POST web-session（幂等，先读比再写 Access）→ 构造 form 原地
// POST 到 {gateway_origin}/auth/sso/redeem → 网关 303 进 ChatGPT。任何一步
// 失败把提示留在 <p> 里，用户关闭弹窗回站点重试即可。
func webSessionLaunchPage(carpoolID int64) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>SurplusToken</title>
</head>
<body>
<p id="msg">…</p>
<script>
(function () {
  var msg = document.getElementById('msg');
  var zh = (navigator.language || '').toLowerCase().indexOf('zh') === 0;
  var text = {
    opening: zh ? '正在打开网页版 ChatGPT…' : 'Opening ChatGPT…',
    failed: zh ? '打开失败，请回到拼车页面重试' : 'Failed to open ChatGPT. Please go back and retry.'
  };
  msg.textContent = text.opening;
  var token = localStorage.getItem('auth_token');
  if (!token) { msg.textContent = text.failed; return; }
  fetch('/api/v1/carpools/%d/web-session', {
    method: 'POST',
    headers: { 'Authorization': 'Bearer ' + token }
  }).then(function (res) {
    if (!res.ok) throw new Error('web-session ' + res.status);
    return res.json();
  }).then(function (body) {
    var session = (body && body.data && body.data.web_session) || {};
    var form = document.createElement('form');
    form.method = 'POST';
    form.action = session.gateway_origin + '/auth/sso/redeem';
    var input = document.createElement('input');
    input.type = 'hidden';
    input.name = 'ticket';
    input.value = session.ticket;
    form.appendChild(input);
    document.body.appendChild(form);
    form.submit();
  }).catch(function () {
    msg.textContent = text.failed;
  });
})();
</script>
</body>
</html>
`, carpoolID)
}
