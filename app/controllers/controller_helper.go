package controllers

import (
	"github.com/gofiber/fiber/v2"
	"strings"
)

func isLoggedIn(c *fiber.Ctx) bool {
	var fromProtected bool
	if protectedValue := c.Locals(FROM_PROTECTED); protectedValue != nil {
		fromProtected = protectedValue.(bool)
	}

	return fromProtected
}

// ExtractUsername gets the username from Locals (set by middleware)
func ExtractUsername(c *fiber.Ctx) string {
	// Get from Locals (set by authentication middleware)
	if userNameValue := c.Locals(USER_NAME); userNameValue != nil {
		if userName, ok := userNameValue.(string); ok {
			return userName
		}
	}

	return ""
}

// GetClientIP determines the actual client IP address considering proxies and dual stack
// Returns both IPv4 and IPv6 addresses if available
func GetClientIP(c *fiber.Ctx) (string, string) {
	// Default values
	ipv4 := ""
	ipv6 := ""

	// 1. Check for Cloudflare header
	cfIP := c.Get("CF-Connecting-IP")
	if cfIP != "" {
		// Cloudflare provides the original client IP in this header
		if strings.Contains(cfIP, ":") {
			// IPv6
			ipv6 = cfIP

			// Look for IPv4 in X-Forwarded-For header as backup
			xffList := strings.Split(c.Get("X-Forwarded-For"), ",")
			if len(xffList) > 0 {
				for _, ip := range xffList {
					ip = strings.TrimSpace(ip)
					if !strings.Contains(ip, ":") {
						// Found IPv4
						ipv4 = ip
						break
					}
				}
			}
		} else {
			// IPv4
			ipv4 = cfIP

			// Look for IPv6 in X-Forwarded-For header as backup
			xffList := strings.Split(c.Get("X-Forwarded-For"), ",")
			if len(xffList) > 0 {
				for _, ip := range xffList {
					ip = strings.TrimSpace(ip)
					if strings.Contains(ip, ":") {
						// Found IPv6
						ipv6 = ip
						break
					}
				}
			}
		}
		return ipv4, ipv6
	}

	// 2. Check for X-Forwarded-For header (standard proxy header)
	xff := c.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For can contain a list of IPs - the first one is the original client IP
		xffList := strings.Split(xff, ",")
		if len(xffList) > 0 {
			// Extract first IP (original client)
			clientIP := strings.TrimSpace(xffList[0])

			// Determine if IPv4 or IPv6
			if strings.Contains(clientIP, ":") {
				ipv6 = clientIP

				// Search list for IPv4
				for i := 1; i < len(xffList); i++ {
					ip := strings.TrimSpace(xffList[i])
					if !strings.Contains(ip, ":") {
						ipv4 = ip
						break
					}
				}
			} else {
				ipv4 = clientIP

				// Search list for IPv6
				for i := 1; i < len(xffList); i++ {
					ip := strings.TrimSpace(xffList[i])
					if strings.Contains(ip, ":") {
						ipv6 = ip
						break
					}
				}
			}

			// If we have both addresses or finished searching the list
			if ipv4 != "" && ipv6 != "" {
				return ipv4, ipv6
			}
		}
	}

	// 3. If no proxy headers were found, use the normal IP address
	ipAddr := c.IP()

	// For ::ffff: IPv4-mapped-IPv6 addresses
	if strings.Contains(ipAddr, ":") {
		// IPv6 address or IPv4 in IPv6 mapping (::ffff:192.168.1.1)
		if strings.Contains(ipAddr, ".") && strings.HasPrefix(ipAddr, "::ffff:") {
			// This is an IPv4 address in IPv6 format
			ipv4 = strings.TrimPrefix(ipAddr, "::ffff:")

			// Try to get a native IPv6 if available
			if realIPv6 := c.Get("X-Real-IP"); realIPv6 != "" && strings.Contains(realIPv6, ":") {
				ipv6 = realIPv6
			}
		} else {
			// This is a pure IPv6 address
			ipv6 = ipAddr

			// Try to get IPv4 from an alternative source
			realIPv4 := c.Get("X-Real-IP")
			if realIPv4 != "" && !strings.Contains(realIPv4, ":") {
				ipv4 = realIPv4
			}
		}
	} else {
		// This is a pure IPv4 address
		ipv4 = ipAddr

		// Try to get IPv6 from an alternative source
		realIPv6 := c.Get("X-Real-IP")
		if realIPv6 != "" && strings.Contains(realIPv6, ":") {
			ipv6 = realIPv6
		}
	}

	return ipv4, ipv6
}
