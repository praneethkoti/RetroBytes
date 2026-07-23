package log

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

type entry struct {
	TS        string         `json:"ts"`
	Level     string         `json:"level"`
	ReqID     string         `json:"req_id,omitempty"`
	IP        string         `json:"ip,omitempty"`
	Method    string         `json:"method,omitempty"`
	Path      string         `json:"path,omitempty"`
	UserID    string         `json:"user_id,omitempty"`
	Action    string         `json:"action,omitempty"`
	Status    int            `json:"status,omitempty"`
	LatencyMs int64          `json:"latency_ms,omitempty"`
	Err       string         `json:"err,omitempty"`
	Fields    map[string]any `json:"fields,omitempty"`
}

func write(level string, c *fiber.Ctx, action string, err error, fields map[string]any) {
	e := entry{TS: time.Now().UTC().Format(time.RFC3339), Level: level, Action: action, Fields: fields}
	if c != nil {
		e.IP = c.IP()
		e.Method = c.Method()
		e.Path = c.Path()
		e.Status = c.Response().StatusCode()
		if rid, ok := c.Locals("requestid").(string); ok && rid != "" {
			e.ReqID = rid
		}
	}
	if err != nil {
		e.Err = err.Error()
	}
	b, _ := json.Marshal(e)
	log.Println(string(b))
}

func Info(c *fiber.Ctx, action string, fields map[string]any) { write("info", c, action, nil, fields) }
func Audit(c *fiber.Ctx, action string, fields map[string]any) {
	write("audit", c, action, nil, fields)
}
func Security(c *fiber.Ctx, action string, fields map[string]any) {
	write("warn", c, action, nil, fields)
}
func Error(c *fiber.Ctx, action string, err error, fields map[string]any) {
	write("error", c, action, err, fields)
}
