# Overview
This is a tool to help you fill in dakoku with very little mouse dependency. All your operations will be via the terminal. Specify a date range, a clock-in time, and a clock-out time — the tool will handle the rest, skipping weekends and holidays automatically.

Happy dakoku-ing!

## Dependencies
- Go 1.23.0+
- Playwright driver and Chromium browser

Install dependencies by running:
```bash
make install_deps
```

## How to use
```bash
make
```

A Chromium browser will open. Log in via the browser, then answer the prompts in the terminal. Type `q` at any prompt to quit.

## Dry run
To test without actually submitting (clicks 戻る instead of 申請):
```bash
make dry_run
```
