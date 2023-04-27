package main

import (
	"context"
	"fmt"
	"strconv"

	"go.uber.org/zap"
	"google.golang.org/api/sheets/v4"
)

func getMembers(ctx context.Context, sheetId string) ([]string, error) {
	srv, err := sheets.NewService(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := srv.Spreadsheets.Values.Get(sheetId, "1:1").Do()
	var members []string
	for k, v := range resp.Values[0] {
		if k != 0 {
			members = append(members, fmt.Sprint(v))
		}
	}
	zap.L().Info("members", zap.Any("members", members))

	return members, nil
}

func getAccounts(ctx context.Context, sheetId string, members []string) ([]account, error) {
	srv, err := sheets.NewService(ctx)
	if err != nil {
		return nil, err
	}

	var accounts []account
	resp, err := srv.Spreadsheets.Values.Get(sheetId, "2:65536").Do()
	for _, row := range resp.Values {
		note := ""
		for i, p := range row {
			if i == 0 {
				note = fmt.Sprint(p)
			} else {
				if p != "" {
					price, err := strconv.Atoi(fmt.Sprint(p))
					if err != nil {
						return nil, err
					}
					accounts = append(accounts, account{User: members[i-1], Price: float64(price), Note: note})
				}
			}
		}
	}

	return accounts, nil
}

func appendAccount(ctx context.Context, sheetId string, rows [][]interface{}) error {
	srv, err := sheets.NewService(ctx)
	if err != nil {
		return err
	}

	vals := &sheets.ValueRange{Values: rows}
	resp, err := srv.Spreadsheets.Values.Append(sheetId, "A2", vals).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do()
	if err != nil {
		return err
	}
	zap.L().Info("append result", zap.Any("response", resp))

	return nil
}
