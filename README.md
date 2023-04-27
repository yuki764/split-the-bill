# split-the-bill

Data is stored in Google Sheets (Google Spreadsheets) and must be followed the format:

| _note | Alice | Bob | ... | Michele |
|:-----:|------:|----:|----:|--------:|
| Train Ticket | 1000  |     |     |         |
| Book  |       | 500 |     |         |
| Cash  | -500  |     |     |     500 |

## Configuration Environment Variables

| `SPREADSHEET_ID`   | (required) the ID of Google Sheets Spreadsheet |
| `HTTP_PATH_PREFIX` | Path prefix to access input form |
