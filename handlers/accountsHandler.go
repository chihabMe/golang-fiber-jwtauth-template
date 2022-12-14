// @Title
// @Description
// @Author
// @Update
package handlers

import (
	"errors"
	"fmt"
	"time"

	"github.com/chihabMe/jwt-auth/core/database"
	"github.com/chihabMe/jwt-auth/core/helpers"
	"github.com/chihabMe/jwt-auth/core/utils"
	"github.com/chihabMe/jwt-auth/models"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"gorm.io/gorm"
)

func getUserByUsername(username string) (*models.User, error) {
	db := database.Instance
	var user models.User
	if err := db.Where(&models.User{Username: username}).Find(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if user.ID == 0 {
		return nil, nil
	}
	return &user, nil

}

func ObtainToken(c *fiber.Ctx) error {
	type LoginInput struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	var input LoginInput
	if err := c.BodyParser(&input); err != nil {
		return c.SendStatus(fiber.StatusUnauthorized)
	}
	username := input.Username
	pass := input.Password
	if username == "" {
		return c.Status(403).JSON(fiber.Map{"status": "failed", "username": "cant be empty"})
	}
	if pass == "" {
		return c.Status(403).JSON(fiber.Map{"status": "failed", "username": "can't be empty"})
	}
	//checking in the database
	user, err := getUserByUsername(username)
	if err != nil || user == nil {
		return c.SendStatus(fiber.StatusUnauthorized)
	}
	same := helpers.CheckPasswordHash(input.Password, user.Password)
	if !same {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "data": "invalid username or password"})
	}

	//
	// token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
	// 	"user_id":  user.ID,
	// 	"username": user.Username,
	// 	"exp":      time.Now().Add(time.Hour * 24).Unix(),
	// })
	//

	//access token
	// token := jwt.New(jwt.SigningMethodHS256)
	// accessClaims := token.Claims.(jwt.MapClaims)
	// accessClaims["user_id"] = user.ID
	// accessClaims["username"] = user.Username
	// accessClaims["exp"] = time.Now().Add(time.Hour * 24).Unix()
	// //refresh token
	// refresh := jwt.New(jwt.SigningMethodES256)
	// refreshClaims := refresh.Claims.(jwt.MapClaims)
	// refreshClaims["user_id"] = user.ID
	// refreshClaims["exp"] = time.Now().Add(time.Hour * 24 * 30).Unix()
	// acc, err := token.SignedString([]byte(config.Config(("SECRET"))))
	// ref, err := token.SignedString([]byte(config.Config(("SECRET"))))
	// if err != nil {
	// 	return c.SendStatus(fiber.StatusUnauthorized)
	// }
	tokens, err := utils.GenerateTokenPair(user)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "data": "server error"})
	}
	c.Cookie(&fiber.Cookie{
		Name:     "Authorization",
		Value:    tokens["access_token"],
		Path:     "/",
		Expires:  time.Now().Add(time.Hour * 24),
		Secure:   false,
		HTTPOnly: true,
	})
	c.Cookie(&fiber.Cookie{
		Name:     "refresh",
		Value:    tokens["refresh_token"],
		Path:     "/",
		Expires:  time.Now().Add(time.Hour * 24 * 30),
		Secure:   false,
		HTTPOnly: true,
	})
	return c.JSON(fiber.Map{"status": "success", "token": fiber.Map{"access": tokens["access_token"], "refresh": tokens["refresh_token"]}})
}
func VerifyToken(c *fiber.Ctx) error {
	// token :=c.Cookies
	jwtData := c.Locals("user").(*jwt.Token)
	claims := jwtData.Claims.(jwt.MapClaims)
	userId := claims["user_id"]
	exp := claims["exp"].(float64)
	fmt.Println(exp)
	var user models.User
	if err := database.Instance.Find(&user, userId).Error; err != nil {
		return c.Status(401).JSON(fiber.Map{"status": "error", "data": "failed"})
	}

	return c.JSON("verify token")
}
func RefreshToken(c *fiber.Ctx) error {
	refreshToken := c.Cookies("refresh")

	token, err := utils.VerifyTokenMethod(refreshToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "data": "failed"})
	}
	alive := utils.VerifyTokenExpireDate(token)
	if !alive {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "data": "dead refresh token"})
	}
	claims := token.Claims.(jwt.MapClaims)

	var user models.User
	if err := database.Instance.First(&user, claims["user_id"]).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "data": "unknown refresh token refresh token"})
	}
	if user.ID == 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "data": "unknown refresh token refresh token"})
	}
	tokens, err := utils.GenerateTokenPair(&user)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "data": "unknown refresh token refresh token"})
	}
	c.Cookie(&fiber.Cookie{
		Name:     "Authorization",
		Value:    tokens["access_token"],
		Path:     "/",
		Expires:  time.Now().Add(time.Hour * 24),
		Secure:   false,
		HTTPOnly: true,
	})
	c.Cookie(&fiber.Cookie{
		Name:     "refresh",
		Value:    tokens["refresh_token"],
		Path:     "/",
		Expires:  time.Now().Add(time.Hour * 24),
		Secure:   false,
		HTTPOnly: true,
	})

	return c.Status(fiber.StatusOK).JSON(nil)
	//return c.Status(fiber.StatusOK).JSON(fiber.Map{"refresh": tokens["refresh_token"], "access": tokens["access_token"]})
}

func RegisterAccount(c *fiber.Ctx) error {
	type UserInput struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	var user models.User
	if err := c.BodyParser(&user); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "please make sure that you didn't miss any field", "data": err})
	}
	errors := models.ValidateUser(user)
	if errors != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "data": errors})
	}
	hash, err := helpers.HashPassword(user.Password)
	if err != nil {
		return c.SendStatus(fiber.StatusUnauthorized)
	}
	user.Password = hash
	if err := database.Instance.Create(&user).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "data": "this username is already being used"})
	}
	newUser := UserInput{
		Email:    user.Email,
		Username: user.Username,
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "infos": "registered", "data": newUser})
}

func Me(c *fiber.Ctx) error {
	type ResponseRes struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Twitter  string `json:"twitter"`
		Github   string `json:"github"`
		LinkeDin string `json:"linkeDin"`
		ID       uint   `json:"id"`
	}
	//token := c.Locals("user").(*jwt.Token)
	//claims := token.Claims.(jwt.MapClaims)
	//username := claims["username"].(string)
	//user, err := getUserByUsername(username)
	user := c.Locals("user").(models.User)

	var res ResponseRes = ResponseRes{}
	res.Username = user.Username
	res.Email = user.Email
	res.Twitter = user.Twitter
	res.Github = user.Github
	res.LinkeDin = user.LinkeDin
	res.ID = user.ID
	return c.JSON(fiber.Map{"status": "success", "data": res})
}
