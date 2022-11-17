package logger

import (
	"fmt"
	"github.com/gofiber/fiber/v2/utils"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/bytebufferpool"
	"github.com/gofiber/fiber/v2/internal/fasttemplate"
)

// Logger variables
const (
	TagPid               = "pid"
	TagTime              = "time"
	TagReferer           = "referer"
	TagProtocol          = "protocol"
	TagPort              = "port"
	TagIP                = "ip"
	TagIPs               = "ips"
	TagHost              = "host"
	TagMethod            = "method"
	TagPath              = "path"
	TagURL               = "url"
	TagUA                = "ua"
	TagLatency           = "latency"
	TagStatus            = "status"
	TagResBody           = "resBody"
	TagReqHeaders        = "reqHeaders"
	TagQueryStringParams = "queryParams"
	TagBody              = "body"
	TagBytesSent         = "bytesSent"
	TagBytesReceived     = "bytesReceived"
	TagRoute             = "route"
	TagError             = "error"
	// DEPRECATED: Use TagReqHeader instead
	TagHeader     = "header:"
	TagReqHeader  = "reqHeader:"
	TagRespHeader = "respHeader:"
	TagLocals     = "locals:"
	TagQuery      = "query:"
	TagForm       = "form:"
	TagCookie     = "cookie:"
	TagBlack      = "black"
	TagRed        = "red"
	TagGreen      = "green"
	TagYellow     = "yellow"
	TagBlue       = "blue"
	TagMagenta    = "magenta"
	TagCyan       = "cyan"
	TagWhite      = "white"
	TagReset      = "reset"
)

const (
	loggerStart = iota
	loggerStop
)

var pid string
var timestamp atomic.Value

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Get timezone location
	tz, err := time.LoadLocation(cfg.TimeZone)
	if err != nil || tz == nil {
		cfg.timeZoneLocation = time.Local
	} else {
		cfg.timeZoneLocation = tz
	}

	// Check if format contains latency
	cfg.enableLatency = strings.Contains(cfg.Format, "${latency}")

	// Create template parser
	tmpl := fasttemplate.New(cfg.Format, "${", "}")

	// Create correct timeformat
	timestamp.Store(time.Now().In(cfg.timeZoneLocation).Format(cfg.TimeFormat))

	// Update date/time every 500 milliseconds in a separate go routine
	if strings.Contains(cfg.Format, "${time}") {
		go func() {
			for {
				time.Sleep(cfg.TimeInterval)
				timestamp.Store(time.Now().In(cfg.timeZoneLocation).Format(cfg.TimeFormat))
			}
		}()
	}

	// Set PID once
	pid = strconv.Itoa(os.Getpid())

	// Set variables
	var (
		once       sync.Once
		mu         sync.Mutex
		errHandler fiber.ErrorHandler
	)

	// If colors are enabled, check terminal compatibility
	if cfg.enableColors {
		cfg.Output = colorable.NewColorableStdout()
		if os.Getenv("TERM") == "dumb" || os.Getenv("NO_COLOR") == "1" || (!isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd())) {
			cfg.Output = colorable.NewNonColorable(os.Stdout)
		}
	}
	errPadding := 15
	errPaddingStr := strconv.Itoa(errPadding)

	tagFunctionMap := cfg.tagFunctions

	// Return new handler
	return func(c *fiber.Ctx) (err error) {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Alias colors
		colors := c.App().Config().ColorScheme

		// Set error handler once
		once.Do(func() {
			// get longested possible path
			stack := c.App().Stack()
			for m := range stack {
				for r := range stack[m] {
					if len(stack[m][r].Path) > errPadding {
						errPadding = len(stack[m][r].Path)
						errPaddingStr = strconv.Itoa(errPadding)
					}
				}
			}
			// override error handler
			errHandler = c.App().ErrorHandler
		})

		var start, stop time.Time

		// Set latency start time
		if cfg.enableLatency {
			start = time.Now()
			c.Context().SetUserValue(loggerStart, start)
		}

		// Handle request, store err for logging
		chainErr := c.Next()

		// Manually call error handler
		if chainErr != nil {
			c.Context().SetUserValue("loggerChainError", chainErr.Error())
			if err := errHandler(c, chainErr); err != nil {
				_ = c.SendStatus(fiber.StatusInternalServerError)
			}
		}

		// Set latency stop time
		if cfg.enableLatency {
			stop = time.Now()
			c.Context().SetUserValue(loggerStop, stop)
		}

		// Get new buffer
		buf := bytebufferpool.Get()

		// Default output when no custom Format or io.Writer is given
		if cfg.enableColors && cfg.Format == ConfigDefault.Format {
			// Format error if exist
			formatErr := ""
			if chainErr != nil {
				formatErr = colors.Red + " | " + chainErr.Error() + colors.Reset
			}

			// Format log to buffer
			_, _ = buf.WriteString(fmt.Sprintf("%s |%s %3d %s| %7v | %15s |%s %-7s %s| %-"+errPaddingStr+"s %s\n",
				timestamp.Load().(string),
				statusColor(c.Response().StatusCode(), colors), c.Response().StatusCode(), colors.Reset,
				stop.Sub(start).Round(time.Millisecond),
				c.IP(),
				methodColor(c.Method(), colors), c.Method(), colors.Reset,
				c.Path(),
				formatErr,
			))

			// Write buffer to output
			_, _ = cfg.Output.Write(buf.Bytes())

			cfg.Done(c, buf.Bytes())

			// Put buffer back to pool
			bytebufferpool.Put(buf)

			// End chain
			return nil
		}

		// Loop over template tags to replace it with the correct value
		_, err = tmpl.ExecuteFunc(buf, func(w io.Writer, tag string) (int, error) {
			if logFunc, ok := tagFunctionMap[tag]; ok {
				return logFunc(buf, c, w, tag)
			}

			if index := strings.Index(tag, ":"); index != -1 {
				result := subStr(tag, 0, index+1)
				logFunc := tagFunctionMap[result]
				return logFunc(buf, c, w, tag)
			}

			return 0, nil
		})
		// Also write errors to the buffer
		if err != nil {
			_, _ = buf.WriteString(err.Error())
		}
		mu.Lock()
		// Write buffer to output
		if _, err := cfg.Output.Write(buf.Bytes()); err != nil {
			// Write error to output
			if _, err := cfg.Output.Write([]byte(err.Error())); err != nil {
				// There is something wrong with the given io.Writer
				fmt.Fprintf(os.Stderr, "Failed to write to log, %v\n", err)
			}
		}
		mu.Unlock()

		cfg.Done(c, buf.Bytes())

		// Put buffer back to pool
		bytebufferpool.Put(buf)

		return nil
	}
}

func appendInt(buf *bytebufferpool.ByteBuffer, v int) (int, error) {
	old := len(buf.B)
	buf.B = fasthttp.AppendUint(buf.B, v)
	return len(buf.B) - old, nil
}

func subStr(str string, start int, length int) (result string) {
	s := utils.UnsafeBytes(str)
	total := len(s)
	if total == 0 {
		return
	}
	// 允许从尾部开始计算
	if start < 0 {
		start = total + start
		if start < 0 {
			return
		}
	}
	if start > total {
		return
	}
	if length < 0 {
		length = total
	}

	end := start + length
	if end > total {
		result = utils.UnsafeString(s[start:])
	} else {
		result = utils.UnsafeString(s[start:end])
	}
	return
}
