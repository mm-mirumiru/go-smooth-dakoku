package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/playwright-community/playwright-go"
)

const (
	CLOCK_IN  string = "i"
	CLOCK_OUT string = "o"
)

var _dryRun bool

func main() {
	flag.BoolVar(&_dryRun, "dry-run", false, "click 戻る instead of 申請 (for testing)")
	flag.Parse()

	if _dryRun {
		fmt.Println("DRY RUN mode active: 戻る will be clicked instead of 申請")
	}

	var d dakoku

	pw, err := playwright.Run()
	if err != nil {
		log.Fatalf("could not start playwright: %v", err)
	}
	defer func() {
		if err = pw.Stop(); err != nil {
			log.Fatalf("could not stop Playwright: %v", err)
		}
	}()

	headless := false
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{Headless: &headless})
	if err != nil {
		log.Fatalf("could not launch browser: %v", err)
	}
	defer func() {
		if err = browser.Close(); err != nil {
			log.Fatalf("could not close browser: %v", err)
		}
	}()

	log.Printf("creating context")
	storageStageFile := `storage.json`
	d.ctx, err = browser.NewContext(playwright.BrowserNewContextOptions{StorageStatePath: &storageStageFile})
	if err != nil {
		d.ctx, err = browser.NewContext()
		if err != nil {
			log.Fatalf("could not create context")
		}
	}

	d.page, err = d.ctx.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}

	d.login()

	for {
		d.openDakokuHistory()

		d.rangeStart = 0
		d.rangeEnd = 0
		d.rangeCurrentDay = 0
		d.rangeClockInTime = ""
		d.rangeClockOutTime = ""

		for d.handleRangeDakoku() {
			d.openDateForm()
			d.clockWithDirectionAndTime(CLOCK_IN, d.rangeClockInTime)
			d.openDateForm()
			d.clockWithDirectionAndTime(CLOCK_OUT, d.rangeClockOutTime)
		}
	}
}

type dakoku struct {
	page              playwright.Page
	ctx               playwright.BrowserContext
	rangeStart        int
	rangeEnd          int
	rangeCurrentDay   int
	rangeClockInTime  string
	rangeClockOutTime string
}

func (d *dakoku) login() {
	if _, err := d.page.Goto("https://id.obc.jp/mx3gddbo4wn1/?manuallogin=True"); err != nil {
		log.Fatalf("could not goto: %v", err)
	}

	log.Printf("waiting for login (up to 3 minutes)")
	timeout := float64(3 * 60 * 1000)
	if err := d.page.GetByRole(playwright.AriaRole(`button`), playwright.PageGetByRoleOptions{Name: `勤務実績`, Exact: &[]bool{true}[0]}).WaitFor(playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateVisible, Timeout: &timeout}); err != nil {
		log.Fatalf("could not log in within 3 minutes: %v", err)
	}

	d.ctx.StorageState("storage.json")
}

func (d *dakoku) openDakokuHistory() {
	log.Printf("clicking 勤務実績")
	if err := d.page.GetByRole(playwright.AriaRole(`button`), playwright.PageGetByRoleOptions{Name: `勤務実績`, Exact: &[]bool{true}[0]}).Click(); err != nil {
		log.Fatalf("could not click 勤務実績: %v", err)
	}

	log.Printf("clicking 勤務実績照会")
	if err := d.page.Locator("#js-p-mb_List").GetByRole(playwright.AriaRole(`button`), playwright.LocatorGetByRoleOptions{Name: `勤務実績照会`}).Click(); err != nil {
		log.Fatalf("could not click 勤務実績照会: %v", err)
	}
}

func (d *dakoku) getAllHolidays() []int {
	elementsOfDateColumn, err := d.page.Locator(`.harcs2-ilr-TableCellCommon`).All()
	if err != nil {
		log.Fatalf("could not get all elements of the date column: %v", err)
	}

	holidays := make([]int, 0)
	for i, element := range elementsOfDateColumn {
		innerTexts, err := element.AllInnerTexts()
		if err != nil {
			log.Fatalf("could not get inner text of date %d: %v", i+1, err)
		}

		if slices.ContainsFunc(innerTexts, func(innerText string) bool {
			return strings.Contains(innerText, "祝") || strings.Contains(innerText, "土") || strings.Contains(innerText, "日")
		}) {
			holidays = append(holidays, i+1)
		}
	}

	return holidays
}

// handleRangeDakoku manages the range-mode flow.
// On the first call it prompts for range and both times, sets rangeCurrentDay to the first working day.
// On subsequent calls it advances rangeCurrentDay to the next working day in the range.
// Returns false when the range is exhausted.
func (d *dakoku) handleRangeDakoku() bool {
	log.Printf("waiting for page to load")
	if err := d.page.Locator(`.cm-p-scrTbl__scrAreaTbl`).WaitFor(playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateVisible}); err != nil {
		log.Fatalf("could not wait for page to load: %v", err)
	}

	if d.rangeCurrentDay == 0 {
		var startInput string
		for {
			fmt.Println("input start date of range, or q to quit. Example: 7 (for the 7th)")
			fmt.Scanln(&startInput)
			if strings.TrimSpace(startInput) == "q" {
				fmt.Println("Goodbye!")
				os.Exit(0)
			}
			if strings.TrimSpace(startInput) != "" {
				break
			}
		}
		start, err := strconv.Atoi(startInput)
		if err != nil {
			log.Fatalf("start date is not an integer")
		}

		var endInput string
		for {
			fmt.Println("input end date of range, or q to quit. Example: 25 (for the 25th)")
			fmt.Scanln(&endInput)
			if strings.TrimSpace(endInput) == "q" {
				fmt.Println("Goodbye!")
				os.Exit(0)
			}
			if strings.TrimSpace(endInput) != "" {
				break
			}
		}
		end, err := strconv.Atoi(endInput)
		if err != nil {
			log.Fatalf("end date is not an integer")
		}

		var clockInInput string
		for {
			fmt.Println("input clock-in time for all dates, or q to quit. Examples:")
			fmt.Println("  0700")
			fmt.Println("  0900")
			fmt.Scanln(&clockInInput)
			if strings.TrimSpace(clockInInput) == "q" {
				fmt.Println("Goodbye!")
				os.Exit(0)
			}
			if strings.TrimSpace(clockInInput) != "" {
				break
			}
		}

		var clockOutInput string
		for {
			fmt.Println("input clock-out time for all dates, or q to quit. Examples:")
			fmt.Println("  1800")
			fmt.Println("  2015")
			fmt.Scanln(&clockOutInput)
			if strings.TrimSpace(clockOutInput) == "q" {
				fmt.Println("Goodbye!")
				os.Exit(0)
			}
			if strings.TrimSpace(clockOutInput) != "" {
				break
			}
		}

		d.rangeStart = start
		d.rangeEnd = end
		d.rangeClockInTime = clockInInput
		d.rangeClockOutTime = clockOutInput

		holidays := d.getAllHolidays()
		day := d.rangeStart
		for slices.Contains(holidays, day) && day <= d.rangeEnd {
			day++
		}
		if day > d.rangeEnd {
			fmt.Println("No working days in specified range.")
			return false
		}
		d.rangeCurrentDay = day
		return true
	}

	holidays := d.getAllHolidays()
	nextDay := d.rangeCurrentDay + 1
	for slices.Contains(holidays, nextDay) && nextDay <= d.rangeEnd {
		nextDay++
	}
	if nextDay > d.rangeEnd {
		fmt.Println("Done! All dates in range have been clocked.")
		return false
	}
	d.rangeCurrentDay = nextDay
	return true
}

func (d *dakoku) openDateForm() {
	log.Printf("waiting for page to load")
	if err := d.page.Locator(`.cm-p-scrTbl__scrAreaTbl`).WaitFor(playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateVisible}); err != nil {
		log.Fatalf("could not wait for page to load: %v", err)
	}

	log.Printf("finding date buttons")
	entries, err := d.page.GetByRole(playwright.AriaRole(`button`), playwright.PageGetByRoleOptions{Name: `申請`}).All()
	if err != nil {
		log.Fatalf("could not get date buttons: %v", err)
	}

	log.Printf("clicking date %d's button", d.rangeCurrentDay)
	if err := entries[d.rangeCurrentDay-1].Click(); err != nil {
		log.Fatalf("could not click date %d's button: %v", d.rangeCurrentDay, err)
	}

	log.Printf("clicking OK button")
	if err := d.page.GetByRole(playwright.AriaRole(`button`), playwright.PageGetByRoleOptions{Name: `OK`}).Click(); err != nil {
		log.Fatalf("could not click OK button: %v", err)
	}
}

func (d *dakoku) clockWithDirectionAndTime(direction, timeInput string) {
	switch direction {
	case CLOCK_IN:
		log.Printf("clicking 出勤 radio")
		if err := d.page.GetByRole(`radio`, playwright.PageGetByRoleOptions{Name: `出勤`}).Click(); err != nil {
			log.Fatalf("could not click 出勤 radio: %v", err)
		}
	case CLOCK_OUT:
		log.Printf("clicking 退勤 radio")
		if err := d.page.GetByRole(`radio`, playwright.PageGetByRoleOptions{Name: `退勤`}).Click(); err != nil {
			log.Fatalf("could not click 退勤 radio: %v", err)
		}
	}

	hour := timeInput[:2]
	minute := timeInput[2:]

	log.Printf("filling in hour")
	if err := d.page.GetByRole(`textbox`, playwright.PageGetByRoleOptions{Name: `時`}).Fill(hour); err != nil {
		log.Fatalf("could not fill in hour: %v", err)
	}

	log.Printf("filling in minute")
	if err := d.page.GetByRole(`textbox`, playwright.PageGetByRoleOptions{Name: `分`}).Fill(minute); err != nil {
		log.Fatalf("could not fill in minute: %v", err)
	}

	d.submitOrGoBack()
}

func (d *dakoku) submitOrGoBack() {
	if _dryRun {
		log.Printf("dry run: clicking 戻る instead of 申請")
		if err := d.page.Locator(`.js-back`).Click(); err != nil {
			log.Fatalf("could not click 戻る button: %v", err)
		}
	} else {
		log.Printf("clicking 申請 button")
		if err := d.page.Locator(`.tm-af-applicationFormBtnArea`).GetByRole(playwright.AriaRole(`button`), playwright.LocatorGetByRoleOptions{Name: `申請`}).Click(); err != nil {
			log.Fatalf("could not click 申請 button: %v", err)
		}
	}
}
