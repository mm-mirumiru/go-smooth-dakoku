# Overview
This is a tool to help you fill in dakoku with very little mouse dependency. All your operations will be via the terminal. The tool will present you questions that require just very short answers. Just answer them and see the tool fill in the dakoku for you.

If you're like me and your clock in and clock out times are almost the same everyday, the tool even remembers your previous input. You might end up just having to hit Enter all the way.

There are 3 modes:
- Enter clock ins only
- Enter clock outs only
- Prompt clock in / clock out for every date

I recommend the first 2 modes for smooth sailing...

Happy dakoku-ing!

## Dependencies
1. Playwright driver and browsers. Install them by running:
```bash
> go run github.com/playwright-community/playwright-go/cmd/playwright@v0.5200.0 install --with-deps
```
1. Go version 1.23.0

## How to use
1. Run the tool in a terminal
```bash
go run .
```
1. A chromium browser will pop up. Put it side by side with the terminal. The tool will ask for your input in the terminal. Interact with the terminal only.
1. Stop the tool at any point with `Ctrl-c`
