package viewmodel

import "github.com/gofiber/fiber/v2"

type Layout struct {
	Page          string
	FromProtected bool
	IsError       bool
	Msg           fiber.Map
	Username      string
	IsAdmin       bool
	OGViewModel   *OpenGraph
}
