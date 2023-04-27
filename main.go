package main

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/exp/slices"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type account struct {
	User  string
	Price float64
	Note  string
}

func main() {
	// adjust fields for GCP Cloud Logging
	// see: https://cloud.google.com/logging/docs/structured-logging
	logCfg := zap.NewProductionConfig()
	logCfg.EncoderConfig.TimeKey = "time"
	logCfg.EncoderConfig.LevelKey = "severity"
	logCfg.EncoderConfig.MessageKey = "message"
	logCfg.EncoderConfig.EncodeTime = zapcore.RFC3339NanoTimeEncoder
	logCfg.EncoderConfig.EncodeLevel = func(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
		switch l {
		case zapcore.DebugLevel:
			enc.AppendString("DEBUG")
		case zapcore.InfoLevel:
			enc.AppendString("INFO")
		case zapcore.WarnLevel:
			enc.AppendString("WARNING")
		case zapcore.ErrorLevel:
			enc.AppendString("ERROR")
		case zapcore.DPanicLevel:
			enc.AppendString("CRITICAL")
		case zapcore.PanicLevel:
			enc.AppendString("ALERT")
		case zapcore.FatalLevel:
			enc.AppendString("EMERGENCY")
		}
	}

	// create logger
	logger, err := logCfg.Build()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	// make logger global
	undo := zap.ReplaceGlobals(logger)
	defer undo()

	// define http handling

	prefix := "/" + os.Getenv("HTTP_PATH_PREFIX") + "/"
	if prefix == "//" {
		prefix = "/"
	}
	zap.L().Info("You must request to " + prefix + "*")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintln(w, "Not Found")
	})

	http.HandleFunc(prefix+"form", renderInputForm)
	http.HandleFunc(prefix+"account", sendAccount)

	if err := http.ListenAndServe(":8080", nil); err == nil {
		zap.L().Fatal(err.Error())
	}
}

// summary: positive means "should send", negative means "should recieve"
func summarizeAccounts(accounts []account, members []string) (float64, map[string]float64) {
	payments := make(map[string]float64)
	for _, m := range members {
		payments[m] = 0.0
	}
	total := 0.0
	for _, a := range accounts {
		total += a.Price
		payments[a.User] += a.Price
	}

	avg := total / float64(len(members))

	summary := make(map[string]float64)

	for u, p := range payments {
		summary[u] = avg - p
	}

	return total, summary
}

func renderInputForm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintln(w, "MethodNotAllowed")
		return
	}

	tpl, err := template.ParseFiles("templates/input-form.go.html")
	if err != nil {
		zap.L().Fatal(err.Error())
	}

	ctx := context.Background()

	sheetId := os.Getenv("SPREADSHEET_ID")
	if sheetId == "" {
		zap.L().Fatal("Spreadsheet ID does not set.")
	}

	members, err := getMembers(ctx, sheetId)
	if err != nil {
		zap.L().Fatal(err.Error())
	}

	accounts, err := getAccounts(ctx, sheetId, members)
	if err != nil {
		zap.L().Fatal(err.Error())
	}

	// if `user` does not specified in Get Parameter, use members[0] as default
	user := r.FormValue("user")
	if !slices.Contains(members, user) {
		user = members[0]
	}

	total, summary := summarizeAccounts(accounts, members)
	zap.L().Info("Total", zap.Float64("total", total))
	zap.L().Info("Summary", zap.Any("summary", summary))

	commaDelim := message.NewPrinter(language.Japanese)
	summaryToRender := make(map[string][2]string)
	for u, p := range summary {
		if p > 0 {
			summaryToRender[u] = [2]string{"💸受け渡し💸", commaDelim.Sprint(p)}
		} else {
			summaryToRender[u] = [2]string{"💰受け取り💰", commaDelim.Sprint(-p)}
		}
	}
	totalToRender := commaDelim.Sprint(total)

	if err := tpl.Execute(w, map[string]interface{}{
		"members":  members,
		"user":     user,
		"accounts": accounts,
		"total":    totalToRender,
		"summary":  summaryToRender,
	}); err != nil {
		zap.L().Fatal(err.Error())
	}
}

func sendAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintln(w, "Method Not Allowed")
		return
	}

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Bad Request")
		return
	}

	ctx := context.Background()

	sheetId := os.Getenv("SPREADSHEET_ID")
	if sheetId == "" {
		zap.L().Fatal("Spreadsheet ID does not set.")
	}

	members, err := getMembers(ctx, sheetId)
	if err != nil {
		zap.L().Fatal(err.Error())
	}

	reciever := ""
	price := 0

	rows := [][]interface{}{{r.PostForm["note"][0]}}

	for k, v := range r.PostForm {
		switch k {
		case "type":
			if strings.HasPrefix(v[0], "transfer/") {
				reciever = strings.Split(v[0], "/")[1]
			}
		case "price":
			price, err = strconv.Atoi(v[0])
			if err != nil {
				zap.L().Fatal(err.Error())
			}
		}
	}
	for _, m := range members {
		if m == r.PostForm["member"][0] {
			rows[0] = append(rows[0], price)
		} else if m == reciever {
			rows[0] = append(rows[0], -price)
		} else {
			rows[0] = append(rows[0], "")
		}
	}

	if err := appendAccount(ctx, sheetId, rows); err != nil {
		zap.L().Fatal(err.Error())
	}

	fmt.Fprintf(w, `<!DOCTYPE html>
<head>
<meta charset="utf-8">
<meta http-equiv="refresh" content="2;URL=form?user=`+r.PostForm["member"][0]+`">
</head>
<body>
<p>送信成功...しばらくお待ち下さい...</p>
</body>
</html>`)
}
