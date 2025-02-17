// Copyright 2021 The Casdoor Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllers

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/beego/beego/logs"
	"github.com/beego/beego/utils/pagination"
	"github.com/casdoor/casdoor/captcha"
	"github.com/casdoor/casdoor/conf"
	"github.com/casdoor/casdoor/object"
	"github.com/casdoor/casdoor/util"
)

type GetEmailAndPhoneResp struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	OneTimeCode string `json:"oneTimeCode"`
}

// GetGlobalUsers
// @Title GetGlobalUsers
// @Tag User API
// @Description get global users
// @Param fillUserIdProvider query bool false "Should fill userIdProvider"
// @Success 200 {array} object.User The Response object
// @Failure 401 Unauthoized
// @Failure 500 Internal server error
// @router /get-global-users [get]
func (c *ApiController) GetGlobalUsers() {
	limitParam := c.Input().Get("pageSize")
	page := c.Input().Get("p")
	field := c.Input().Get("field")
	value := c.Input().Get("value")
	sortField := c.Input().Get("sortField")
	sortOrder := c.Input().Get("sortOrder")
	fillUserIdProvider := util.ParseBool(c.Input().Get("fillUserIdProvider"))

	if _, res := c.RequireAdmin(); !res {
		c.ResponseUnauthorized(c.T("auth:Unauthorized operation"))
		return
	}

	var limit int
	if limitParam == "" || page == "" {
		limit = -1
	} else {
		limit = max(100, util.ParseInt(limitParam))
	}

	count, err := object.GetGlobalUserCount(field, value)
	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}

	paginator := pagination.SetPaginator(c.Ctx, limit, count)
	users, err := object.GetPaginationGlobalUsers(paginator.Offset(), limit, field, value, sortField, sortOrder)
	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}

	users, err = object.GetMaskedUsers(users)
	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}

	if fillUserIdProvider {
		userIdProviders, err := object.GetGlobalUserIdProviders()
		if err != nil {
			c.ResponseError(err.Error())
			return
		}

		fillUserIdProviders(users, userIdProviders)
	}

	c.ResponseOk(users, paginator.Nums())
}

// GetUsers
// @Title GetUsers
// @Tag User API
// @Description
// @Param owner query string true "The owner of users"
// @Param fillUserIdProvider query bool false "Should fill userIdProvider"
// @Success 200 {array} object.User The Response object
// @Failure 500 Internal server error
// @router /get-users [get]
func (c *ApiController) GetUsers() {
	owner := c.Input().Get("owner")
	groupName := c.Input().Get("groupName")
	limitParam := c.Input().Get("pageSize")
	page := c.Input().Get("p")
	field := c.Input().Get("field")
	value := c.Input().Get("value")
	sortField := c.Input().Get("sortField")
	sortOrder := c.Input().Get("sortOrder")
	fillUserIdProvider := util.ParseBool(c.Input().Get("fillUserIdProvider"))

	var limit int
	if limitParam == "" || page == "" {
		limit = -1
	} else {
		limit = max(100, util.ParseInt(limitParam))
	}

	count, err := object.GetUserCount(owner, field, value, groupName)
	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}

	paginator := pagination.SetPaginator(c.Ctx, limit, count)
	users, err := object.GetPaginationUsers(owner, paginator.Offset(), limit, field, value, sortField, sortOrder, groupName)
	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}

	users, err = object.GetMaskedUsers(users)
	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}

	if fillUserIdProvider {
		userIdProviders, err := object.GetUserIdProviders(owner)
		if err != nil {
			c.ResponseInternalServerError(err.Error())
			return
		}

		fillUserIdProviders(users, userIdProviders)
	}

	c.ResponseOk(users, paginator.Nums())
}

// GetUser
// @Title GetUser
// @Tag User API
// @Description get user
// @Param   id               query    string  false        "The id ( owner/name ) of the user"
// @Param   owner            query    string  false        "The owner of the user"
// @Param   email            query    string  false 	   "The email of the user"
// @Param   phone            query    string  false 	   "The phone of the user"
// @Param   userId           query    string  false 	   "The userId of the user"
// @Param   fillUserIdProvider query    bool    false        "Should fill userIdProvider"
// @Success 200 {object} object.User The Response object
// @Failure 401 Unauthorized
// @Failure 404 Not found
// @Failure 500 Internal server error
// @router /get-user [get]
func (c *ApiController) GetUser() {
	id := c.Input().Get("id")
	email := c.Input().Get("email")
	phone := c.Input().Get("phone")
	userId := c.Input().Get("userId")
	owner := c.Input().Get("owner")
	fillUserIdProvider := util.ParseBool(c.Input().Get("fillUserIdProvider"))

	var err error
	var user *object.User
	switch {
	case email != "":
		user, err = object.GetUserByEmail(owner, email)
	case phone != "":
		user, err = object.GetUserByPhone(owner, phone)
	case userId != "":
		user, err = object.GetUserByUserId(owner, userId)
	default:
		user, err = object.GetUser(id)
	}

	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}
	if user == nil {
		c.ResponseNotFound("user not found")
		return
	}

	id = util.GetId(user.Owner, user.Name)

	if owner == "" {
		owner = util.GetOwnerFromId(id)
	}

	organization, err := object.GetOrganization(util.GetId("admin", owner))
	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}

	if !organization.IsProfilePublic {
		requestUserId := c.GetSessionUsername()
		hasPermission, err := object.CheckUserPermission(requestUserId, id, false, c.GetAcceptLanguage())
		if _, ok := err.(*object.NotFoundError); ok {
			c.ResponseNotFound(err.Error())
			return
		}

		if !hasPermission {
			c.ResponseUnauthorized(err.Error())
			return
		}
	}

	user.MultiFactorAuths = object.GetAllMfaProps(user, true)

	err = object.ExtendUserWithRolesAndPermissions(user)
	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}

	isAdminOrSelf := c.IsAdminOrSelf(user)
	maskedUser, err := object.GetMaskedUser(user, isAdminOrSelf)
	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}

	if fillUserIdProvider {
		userIdProviders, err := object.GetUserIdProviders(util.GetOwnerFromId(id))
		if err != nil {
			c.ResponseInternalServerError(err.Error())
			return
		}

		fillUserIdProviders([]*object.User{maskedUser}, userIdProviders)
	}

	c.ResponseOk(maskedUser)
}

// AddUserIdProvider
// @Title AddUserIdProvider
// @Tag User API
// @Description add user id provider
// @Param   body    body   object.UserIdProvider  true        "The details of the user id provider"
// @Success 200 {object} controllers.Response The Response object
// @router /add-user-id-provider [post]
func (c *ApiController) AddUserIdProvider() {
	goCtx := c.getRequestCtx()
	record := object.GetRecord(goCtx)

	var userIdProvider object.UserIdProvider
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &userIdProvider)
	if err != nil {
		c.ResponseBadRequest(err.Error())
		return
	}

	if !c.IsGlobalAdmin() {
		c.ResponseUnauthorized(c.T("auth:Unauthorized operation"))
		return
	}

	if userIdProvider.Owner == "" || !object.CheckUserIdProviderOrigin(userIdProvider) || userIdProvider.UsernameFromIdp == "" {
		record.AddReason("Add UserIdProvider: Failed to add userIdProvider. Missing parameter")
		c.ResponseUnprocessableEntity(c.T("general:Missing parameter"))
		return
	}

	userIdProvider.CreatedTime = util.GetCurrentTime()

	affected, err := object.AddUserIdProvider(c.Ctx.Request.Context(), &userIdProvider)
	if err != nil || !affected {
		record.AddReason("Add UserIdProvider: Failed to add userIdProvider")
		c.ResponseInternalServerError(c.T("user:Failed to add userIdProvider"))
		return
	}

	c.Data["json"] = wrapActionResponse(affected)
	c.ServeJSON()
}

// UpdateUser
// @Title UpdateUser
// @Tag User API
// @Description update user
// @Param   id     query    string  true        "The id ( owner/name ) of the user"
// @Param   body    body   object.User  true        "The details of the user"
// @Success 200 {object} controllers.Response The Response object
// @Failure 400 Bad request
// @Failure 403 Forbidden
// @Failure 404 Not found
// @Failure 422 Unprocessable entity
// @Failure 500 Internal server error
// @router /update-user [post]
func (c *ApiController) UpdateUser() {
	id := c.Input().Get("id")
	columnsStr := c.Input().Get("columns")

	goCtx := c.getRequestCtx()
	record := object.GetRecord(goCtx)

	var user object.User
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &user)
	if err != nil {
		c.ResponseBadRequest(err.Error())
		return
	}

	if id == "" {
		id = c.GetSessionUsername()
		if id == "" {
			c.ResponseUnprocessableEntity(c.T("general:Missing parameter"))
			return
		}
	}

	oldUser, err := object.GetUser(id)
	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}

	if oldUser == nil {
		c.ResponseNotFound(fmt.Sprintf(c.T("general:The user: %s doesn't exist"), id))
		return
	}

	if oldUser.Owner == "built-in" && oldUser.Name == "admin" && (user.Owner != "built-in" || user.Name != "admin") {
		c.ResponseForbidden(c.T("auth:Unauthorized operation"))
		return
	}

	if c.Input().Get("allowEmpty") == "" {
		if user.DisplayName == "" {
			c.ResponseInternalServerError(c.T("user:Display name cannot be empty"))
			return
		}
	}

	if msg := object.CheckUpdateUser(oldUser, &user, c.GetAcceptLanguage()); msg != "" {
		c.ResponseUnprocessableEntity(msg)
		return
	}

	isAdmin := c.IsAdmin()
	if pass, err := object.CheckPermissionForUpdateUser(oldUser, &user, isAdmin, c.GetAcceptLanguage()); !pass {
		c.ResponseForbidden(err)
		return
	}

	columns := []string{}
	if columnsStr != "" {
		columns = strings.Split(columnsStr, ",")
	}

	affected, err := object.UpdateUser(id, &user, columns, isAdmin)
	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}

	if affected {
		err = object.UpdateUserToOriginalDatabase(&user)
		if err != nil {
			c.ResponseInternalServerError(err.Error())
			return
		}
	}

	record.AddOldObject(oldUser).AddReason("Update user")

	c.Data["json"] = wrapActionResponse(affected)
	c.ServeJSON()
}

// AddUser
// @Title AddUser
// @Tag User API
// @Description add user
// @Param   body    body   object.User  true        "The details of the user"
// @Success 200 {object} controllers.Response The Response object
// @Failure 400 Bad request
// @Failure 422 Unprocessable entity
// @Failure 500 Internal server error
// @router /add-user [post]
func (c *ApiController) AddUser() {
	var user object.User
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &user)
	if err != nil {
		c.ResponseBadRequest(err.Error())
		return
	}

	count, err := object.GetUserCount("", "", "", "")
	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}

	if err := checkQuotaForUser(int(count)); err != nil {
		c.ResponseUnprocessableEntity(err.Error())
		return
	}

	msg := object.CheckUsername(user.Name, c.GetAcceptLanguage())
	if msg != "" {
		c.ResponseUnprocessableEntity(msg)
		return
	}

	c.Data["json"] = wrapActionResponse(object.AddUser(&user))
	c.ServeJSON()
}

// DeleteUser
// @Title DeleteUser
// @Tag User API
// @Description delete user
// @Param   body    body   object.User  true        "The details of the user"
// @Success 200 {object} controllers.Response The Response object
// @Failure 400 Bad request
// @Failure 403 Forbidden
// @router /delete-user [post]
func (c *ApiController) DeleteUser() {
	var user object.User
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &user)
	if err != nil {
		c.ResponseBadRequest(err.Error())
		return
	}

	if user.Owner == "built-in" && user.Name == "admin" {
		c.ResponseForbidden(c.T("auth:Unauthorized operation"))
		return
	}

	c.Data["json"] = wrapActionResponse(object.DeleteUser(&user))
	c.ServeJSON()
}

// GetEmailAndPhone
// @Title GetEmailAndPhone
// @Tag User API
// @Description get email and phone by username
// @Param   username    formData   string  true        "The username of the user"
// @Param   organization    formData   string  true        "The organization of the user"
// @Success 200 {object} controllers.Response The Response object
// @Failure 404 Not found
// @Failure 422 Unprocessable entity
// @Failure 500 Internal server error
// @router /get-email-and-phone [get]
func (c *ApiController) GetEmailAndPhone() {
	organization := c.Ctx.Request.Form.Get("organization")
	applicationId := c.Ctx.Request.Form.Get("applicationId")
	username := c.Ctx.Request.Form.Get("username")
	clientSecret := c.Ctx.Request.Form.Get("captchaCode")
	captchaToken := c.Ctx.Request.Form.Get("captchaToken")

	applicationCaptchaProvider, err := object.GetCaptchaProviderByApplication(applicationId, "false", c.GetAcceptLanguage())
	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}

	if applicationCaptchaProvider != nil {
		if captchaProvider := captcha.GetCaptchaProvider(applicationCaptchaProvider.Type); captchaProvider == nil {
			c.ResponseInternalServerError(c.T("general:don't support captchaProvider: ") + applicationCaptchaProvider.Type)
			return
		} else if isHuman, err := captchaProvider.VerifyCaptcha(captchaToken, clientSecret); err != nil {
			c.ResponseUnprocessableEntity(err.Error())
			return
		} else if !isHuman {
			c.ResponseUnprocessableEntity(c.T("verification:Incorrect input of captcha characters."))
			return
		}
	}

	user, err := object.GetUserByFields(organization, username)
	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}

	if user == nil || user.Type == "invited-user" {
		c.ResponseNotFound(fmt.Sprintf(c.T("general:The user: %s doesn't exist"), util.GetId(organization, username)))
		return
	}

	oneTimeCode := util.GetRandomCode(6)

	c.SetSession("oneTimeCode", oneTimeCode)

	respUser := GetEmailAndPhoneResp{Name: user.Name, OneTimeCode: oneTimeCode}
	var contentType string
	switch username {
	case user.Email:
		contentType = "email"
		respUser.Email = user.Email
	case user.Phone:
		contentType = "phone"
		respUser.Phone = user.Phone
	case user.Name:
		contentType = "username"
		respUser.Email = util.GetMaskedEmail(user.Email)
		respUser.Phone = util.GetMaskedPhone(user.Phone)
	}

	c.ResponseOk(respUser, contentType)
}

// SetPassword
// @Title SetPassword
// @Tag Account API
// @Description set password
// @Param   userOwner   formData    string  true        "The owner of the user"
// @Param   userName   formData    string  true        "The name of the user"
// @Param   oldPassword   formData    string  true        "The old password of the user"
// @Param   newPassword   formData    string  true        "The new password of the user"
// @Success 200 {object} controllers.Response The Response object
// @Failure 401 Unauthorized
// @Failure 403 Forbidden
// @Failure 404 Not found
// @Failure 422 Unprocessable entity
// @Failure 500 Internal server error
// @router /set-password [post]
func (c *ApiController) SetPassword() {
	userOwner := c.Ctx.Request.Form.Get("userOwner")
	userName := c.Ctx.Request.Form.Get("userName")
	oldPassword := c.Ctx.Request.Form.Get("oldPassword")
	newPassword := c.Ctx.Request.Form.Get("newPassword")
	code := c.Ctx.Request.Form.Get("code")

	//if userOwner == "built-in" && userName == "admin" {
	//	c.ResponseError(c.T("auth:Unauthorized operation"))
	//	return
	//}

	if strings.Contains(newPassword, " ") {
		c.ResponseUnprocessableEntity(c.T("user:New password cannot contain blank space."))
		return
	}

	userId := util.GetId(userOwner, userName)

	requestUserId := c.GetSessionUsername()

	fromChangePasswordRequiredForm := requestUserId == "" && c.getChangePasswordUserSession() != ""

	if fromChangePasswordRequiredForm {
		requestUserId = c.getChangePasswordUserSession()
	}

	if requestUserId == "" && code == "" {
		c.ResponseUnauthorized(c.T("general:Please login first"))
		return
	} else if code == "" {
		hasPermission, err := object.CheckUserPermission(requestUserId, userId, true, c.GetAcceptLanguage())
		if !hasPermission {
			c.ResponseForbidden(err.Error())
			return
		}
	} else {
		if code != c.GetSession("verifiedCode") || userId != c.GetSession("verifiedUserId") {
			c.ResponseUnprocessableEntity(c.T("general:Missing parameter"))
			return
		}
		c.SetSession("verifiedCode", "")
		c.SetSession("verifiedUserId", "")
	}

	targetUser, err := object.GetUser(userId)
	if targetUser == nil {
		c.ResponseNotFound(fmt.Sprintf(c.T("general:The user: %s doesn't exist"), userId))
		return
	}
	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}

	if targetUser.Type == "invited-user" {
		c.ResponseForbidden(c.T("auth:Unauthorized operation"))
		return
	}

	isAdmin := c.IsAdmin()
	if isAdmin {
		if oldPassword != "" {
			err := object.CheckPassword(targetUser, oldPassword, c.GetAcceptLanguage())
			if err != nil {
				c.ResponseUnauthorized(err.Error())
				return
			}
		}
	} else if code == "" {
		err := object.CheckPassword(targetUser, oldPassword, c.GetAcceptLanguage())
		if err != nil {
			c.ResponseUnauthorized(err.Error())
			return
		}
	}

	msg := object.CheckPasswordComplexity(targetUser, newPassword, c.GetAcceptLanguage())
	if msg != "" {
		c.ResponseUnprocessableEntity(msg)
		return
	}

	msg = object.CheckPasswordSame(targetUser, newPassword, c.GetAcceptLanguage())
	if msg != "" {
		c.ResponseUnprocessableEntity(msg)
		return
	}

	targetUser.Password = newPassword
	_, err = object.SetUserField(targetUser, "password", targetUser.Password)
	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}

	c.ResponseOk()
}

// CheckUserPassword
// @Title CheckUserPassword
// @router /check-user-password [post]
// @Failure 400 Bad request
// @Failure 401 Unauthorized
// @Tag User API
func (c *ApiController) CheckUserPassword() {
	var user object.User
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &user)
	if err != nil {
		c.ResponseBadRequest(err.Error())
		return
	}

	_, err = object.CheckUserPassword(user.Owner, user.Name, user.Password, c.GetAcceptLanguage())
	if err == nil {
		c.ResponseOk()
	} else {
		msg := object.CheckPassErrorToMessage(err, c.GetAcceptLanguage())
		c.ResponseUnauthorized(msg)
	}
}

// GetSortedUsers
// @Title GetSortedUsers
// @Tag User API
// @Description
// @Param   owner     query    string  true        "The owner of users"
// @Param   sorter     query    string  true        "The DB column name to sort by, e.g., created_time"
// @Param   limit     query    string  true        "The count of users to return, e.g., 25"
// @Success 200 {array} object.User The Response object
// @Failure 500 Internal server error
// @router /get-sorted-users [get]
func (c *ApiController) GetSortedUsers() {
	owner := c.Input().Get("owner")
	sorter := c.Input().Get("sorter")
	limit := util.ParseInt(c.Input().Get("limit"))

	maskedUsers, err := object.GetMaskedUsers(object.GetSortedUsers(owner, sorter, limit))
	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}

	c.ResponseOk(maskedUsers)
}

// GetUserCount
// @Title GetUserCount
// @Tag User API
// @Description
// @Param   owner     query    string  true        "The owner of users"
// @Param   isOnline     query    string  true        "The filter for query, 1 for online, 0 for offline, empty string for all users"
// @Success 200 {int} int The count of filtered users for an organization
// @Failure 500 Internal server error
// @router /get-user-count [get]
func (c *ApiController) GetUserCount() {
	owner := c.Input().Get("owner")
	isOnline := c.Input().Get("isOnline")

	var count int64
	var err error
	if isOnline == "" {
		count, err = object.GetUserCount(owner, "", "", "")
	} else {
		count, err = object.GetOnlineUserCount(owner, util.ParseInt(isOnline))
	}
	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}

	c.ResponseOk(count)
}

// AddUserkeys
// @Title AddUserkeys
// @router /add-user-keys [post]
// @Failure 400 Bad request
// @Failure 500 Internal server error
// @Tag User API
func (c *ApiController) AddUserkeys() {
	var user object.User
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &user)
	if err != nil {
		c.ResponseBadRequest(err.Error())
		return
	}

	isAdmin := c.IsAdmin()
	affected, err := object.AddUserkeys(&user, isAdmin)
	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}

	c.ResponseOk(affected)
}

// RemoveUserFromGroup
// @Title RemoveUserFromGroup
// @router /remove-user-from-group [post]
// @Failure 500 Internal server error
// @Tag User API
func (c *ApiController) RemoveUserFromGroup() {
	owner := c.Ctx.Request.Form.Get("owner")
	name := c.Ctx.Request.Form.Get("name")
	groupName := c.Ctx.Request.Form.Get("groupName")

	organization, err := object.GetOrganization(util.GetId("admin", owner))
	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}
	item := object.GetAccountItemByName("Groups", organization)
	res, msg := object.CheckAccountItemModifyRule(item, c.IsAdmin(), c.GetAcceptLanguage())
	if !res {
		c.ResponseInternalServerError(msg)
		return
	}

	affected, err := object.DeleteGroupForUser(util.GetId(owner, name), groupName)
	if err != nil {
		c.ResponseInternalServerError(err.Error())
		return
	}

	c.ResponseOk(affected)
}

// SendInvite
// @Title SendInvite
// @router /send-invite [post]
// @Failure 401 Unauthorized
// @Failure 422 Unprocessable entity
// @Failure 500 Internal server error
// @Tag User API
func (c *ApiController) SendInvite() {
	owner := c.Input().Get("owner")
	username := c.Input().Get("name")

	if !c.IsAdmin() {
		c.ResponseUnauthorized(c.T("auth:Unauthorized operation"))
		return
	}

	user, err := object.GetUser(util.GetId(owner, username))
	if err != nil {
		logs.Error("get user: %s", err.Error())
		c.ResponseInternalServerError("internal server error")

		return
	}

	if user.Email == "" {
		c.ResponseUnprocessableEntity(fmt.Sprintf(c.T("service:Missing email for send invite")))
		return
	}

	if !util.IsEmailValid(user.Email) {
		c.ResponseUnprocessableEntity(fmt.Sprintf(c.T("service:Invalid Email for send invite: %s"), user.Email))
		return
	}

	organization, err := object.GetOrganization(util.GetId("admin", owner))
	if err != nil {
		logs.Error("get organization: %s", err.Error())
		c.ResponseInternalServerError("internal server error")

		return
	}

	application, err := object.GetApplicationByUser(user)
	if err != nil {
		logs.Error("get application by user: %s", err.Error())
		c.ResponseInternalServerError("internal server error")

		return
	}

	provider, err := application.GetEmailProvider()
	if err != nil {
		logs.Error("get email provider: %s", err.Error())
		c.ResponseInternalServerError("internal server error")

		return
	}

	if provider == nil {
		c.ResponseUnprocessableEntity(c.T("service:Please set an Email provider first"))
		return
	}

	sender := organization.DisplayName
	origin := conf.GetConfigString("origin")

	var link string
	switch user.Type {
	case "invited-user":
		link = fmt.Sprintf("%s/signup/%s?id=%s&u=%s&e=%s", origin, application.Name, user.Id, user.Name, user.Email)
	default:
		switch {
		case application.Name == "app-built-in":
			link = fmt.Sprintf("%s/login?u=%s", origin, user.Name)
		case application.SigninUrl != "":
			link = application.SigninUrl
		default:
			if len(application.RedirectUris) == 0 {
				c.ResponseUnprocessableEntity(c.T("service:You must specify at least one Redirect URL"))
				return
			}
			link = fmt.Sprintf("%s/login/oauth/authorize?client_id=%s&response_type=code&redirect_uri=%s&scope=read&state=casgate", origin, application.ClientId, application.RedirectUris[0])
		}
	}

	content, err := url.JoinPath(provider.InviteContent, link)
	if err != nil {
		logs.Error("join path: %s", err.Error())
		c.ResponseInternalServerError("internal server error")

		return
	}

	err = object.SendEmail(provider, provider.InviteTitle, content, user.Email, sender)
	if err != nil {
		logs.Error("send email: %s", err.Error())
		c.ResponseInternalServerError("internal server error")

		return
	}

	c.ResponseOk()
}

func fillUserIdProviders(users []*object.User, userIdProviders []*object.UserIdProvider) {
	userIdProviderMap := make(map[string]*object.UserIdProvider)
	for i := range userIdProviders {
		userIdProviderMap[userIdProviders[i].UserId] = userIdProviders[i]
	}

	for i := range users {
		if userIdProvider, ok := userIdProviderMap[users[i].Id]; ok {
			users[i].UserIdProvider = userIdProvider
		}
	}
}
