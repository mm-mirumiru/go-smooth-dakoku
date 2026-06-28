package main

import (
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

const (
	CLOCK_IN      string = "i"
	CLOCK_OUT     string = "o"
	ASK_EVERYTIME string = "a"
)

var (
	_inout    string
	_prevDay  string
	_prevTime string
)

func main() {
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
		fmt.Println("What do you want to do?")
		fmt.Println("  type i for 'clock in the same time for a range of date'")
		fmt.Println("  type o for 'clock out the same time for a range of date'")
		fmt.Println("  type a for 'ask me again for every date'")
		fmt.Scanln(&_inout)

		_prevDay = ""
		d.rangeStart = 0
		d.rangeEnd = 0
		d.rangeCurrentDay = 0
		d.rangeTime = ""

		d.openDakokuHistory()

		for d.handleRangeDakoku() {
			d.promptUserForDateToClock()
			d.promptUserToChooseClockInOrOut()
			d.promptUserToInputTime()
		}
	}
}

type dakoku struct {
	page           playwright.Page
	ctx            playwright.BrowserContext
	rangeStart     int
	rangeEnd       int
	rangeCurrentDay int
	rangeTime      string
}

func (d *dakoku) login() {
	if _, err := d.page.Goto("https://id.obc.jp/mx3gddbo4wn1/?manuallogin=True"); err != nil {
		log.Fatalf("could not goto: %v", err)
	}

	// If already logged in due to cookies, the page jumps to timerecorder, and URL is different.
	// ID input element is not present.
	// So, skip entering ID
	if d.page.URL() == "https://id.obc.jp/mx3gddbo4wn1/?manuallogin=True" {
		log.Printf("finding ID input")
		entries, err := d.page.Locator(".inputText").All()
		if err != nil {
			log.Fatalf("could not get entries: %v", err)
		}

		idArea := entries[0]
		log.Printf("getting ID input text")
		id, err := idArea.InnerText()
		if err != nil {
			log.Fatalf("could not get ID input text")
		}
		log.Printf("got it. It's '%s'", id)

		if id == "" {
			var input string
			fmt.Println("input your dakoku ID. Example:")
			fmt.Println("  mirumiru@macromill.com")
			fmt.Scanln(&input)

			log.Printf("typing ID")
			if err := idArea.Fill(input); err != nil {
				log.Fatalf("could not type in ID area: %v", err)
			}

			log.Printf("pressing Enter to proceed")
			if err := d.page.Keyboard().Press(`Enter`); err != nil {
				log.Fatalf("could not type Enter: %v", err)
			}
		} else {
			log.Printf("clicking ID input to proceed")
			if err := idArea.Click(); err != nil {
				log.Fatalf("could not click ID input: %v", err)
			}
		}

	}

	log.Printf("waiting for page to load")
	if err := d.page.GetByRole(playwright.AriaRole(`button`), playwright.PageGetByRoleOptions{Name: `勤務実績`, Exact: &[]bool{true}[0]}).WaitFor(playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateVisible}); err != nil {
		log.Fatalf("could not wait for page to load: %v", err)
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

// handleRangeDakoku manages the range-mode flow for i/o.
// On the first call it prompts for range and time, sets rangeCurrentDay to the first working day.
// On subsequent calls it advances rangeCurrentDay to the next working day in the range.
// Returns false when the range is exhausted (caller should break the loop).
// In ASK_EVERYTIME mode it is a no-op and always returns true.
func (d *dakoku) handleRangeDakoku() bool {
	if _inout == ASK_EVERYTIME {
		return true
	}

	log.Printf("waiting for page to load")
	if err := d.page.Locator(`.cm-p-scrTbl__scrAreaTbl`).WaitFor(playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateVisible}); err != nil {
		log.Fatalf("could not wait for page to load: %v", err)
	}

	if d.rangeCurrentDay == 0 {
		var startInput string
		for {
			fmt.Println("input start date of range. Example: 7 (for the 7th)")
			fmt.Scanln(&startInput)
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
			fmt.Println("input end date of range. Example: 25 (for the 25th)")
			fmt.Scanln(&endInput)
			if strings.TrimSpace(endInput) != "" {
				break
			}
		}
		end, err := strconv.Atoi(endInput)
		if err != nil {
			log.Fatalf("end date is not an integer")
		}

		var timeInput string
		for {
			fmt.Println("input time for all dates. Examples:")
			fmt.Println("  0700")
			fmt.Println("  1815")
			fmt.Scanln(&timeInput)
			if strings.TrimSpace(timeInput) != "" {
				break
			}
		}

		d.rangeStart = start
		d.rangeEnd = end
		d.rangeTime = timeInput

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

func (d *dakoku) promptUserForDateToClock() {
	var dayToClick int

	if _inout == ASK_EVERYTIME {
		log.Printf("waiting for page to load")
		if err := d.page.Locator(`.cm-p-scrTbl__scrAreaTbl`).WaitFor(playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateVisible}); err != nil {
			log.Fatalf("could not wait for page to load: %v", err)
		}

		var input string
		var nextDay string
		for {
			fmt.Println("type date to access. Just the day number is enough. Examples: ")
			fmt.Println("  7 (for the 7th day of the month)")
			fmt.Println("  14 (for the 14th day of the month)")
			if _prevDay != "" {
				nextDay = d.getNextWorkingDay(_prevDay)
				if nextDay != "" {
					fmt.Printf("  %s (next working day. using this if input is empty)", nextDay)
				}
			}
			fmt.Scanln(&input)
			if strings.TrimSpace(input) == "" {
				if nextDay == "" {
					fmt.Println("no input given. prompting again")
					continue
				}

				input = nextDay
				_prevDay = input
				break
			}

			_prevDay = input
			break
		}

		i, err := strconv.Atoi(input)
		if err != nil {
			log.Fatalf("date to access is not an integer")
		}
		dayToClick = i
	} else {
		log.Printf("processing day %d of range", d.rangeCurrentDay)
		dayToClick = d.rangeCurrentDay
	}

	log.Printf("finding date buttons")
	entries, err := d.page.GetByRole(playwright.AriaRole(`button`), playwright.PageGetByRoleOptions{Name: `申請`}).All()
	if err != nil {
		log.Fatalf("could not get : %v", err)
	}

	log.Printf("clicking date %d's button", dayToClick)
	if err := entries[dayToClick-1].Click(); err != nil {
		log.Fatalf("could not click date %d's button: %v", dayToClick, err)
	}

	log.Printf("clicking OK button")
	if err := d.page.GetByRole(playwright.AriaRole(`button`), playwright.PageGetByRoleOptions{Name: `OK`}).Click(); err != nil {
		log.Fatalf("could not click OK button: %v", err)
	}
}

// getNextWorkingDay returns the next working day since fromString in the same month.
// It returns "" if there is no next working day in the same month
func (d *dakoku) getNextWorkingDay(fromString string) string {
	now := time.Now()
	firstOfNextMonth := time.Date(
		now.Year(),
		now.Month()+1,
		1,
		0, 0, 0, 0,
		now.Local().Location(),
	)
	lastOfThisMonth := firstOfNextMonth.AddDate(0, 0, -1)
	maxDaysThisMonth := lastOfThisMonth.Day()
	from, err := strconv.Atoi(fromString)
	if err != nil {
		log.Fatalf("date to access is not an integer")
	}
	current := time.Date(now.Year(), now.Month(), from, 0, 0, 0, 0, now.Local().Location())

	holidays := d.getAllHolidays()
	nextDay := current.Day() + 1
	for slices.Contains(holidays, nextDay) {
		nextDay += 1
	}

	if nextDay > maxDaysThisMonth {
		return ""
	}

	return fmt.Sprintf("%d", nextDay)
}

func (d *dakoku) promptUserToChooseClockInOrOut() {
	var input string
	if _inout == ASK_EVERYTIME {
		fmt.Println("choose clock in or out:")
		fmt.Println("  type i for in")
		fmt.Println("  type o for out")
		fmt.Scanln(&input)
	} else {
		input = _inout
	}

	switch input {
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
	default:
		// do nothing
	}
}

func (d *dakoku) promptUserToInputTime() {
	var input string

	if _inout == ASK_EVERYTIME {
		for {
			fmt.Println("input time. Examples:")
			fmt.Println("  0700")
			fmt.Println("  1815")
			if _prevTime != "" {
				fmt.Printf("  %s (previously set. using this if input is empty)\n", _prevTime)
			}
			fmt.Scanln(&input)

			if strings.TrimSpace(input) == "" {
				if _prevTime == "" {
					fmt.Println("no input given. prompting again")
					continue
				}

				input = _prevTime
				break
			}

			_prevTime = input
			break
		}
	} else {
		input = d.rangeTime
	}

	hour := input[:2]
	minute := input[2:]

	log.Printf("filling in hour")
	if err := d.page.GetByRole(`textbox`, playwright.PageGetByRoleOptions{Name: `時`}).Fill(hour); err != nil {
		log.Fatalf("could not fill in hour: %v", err)
	}

	log.Printf("filling in minute")
	if err := d.page.GetByRole(`textbox`, playwright.PageGetByRoleOptions{Name: `分`}).Fill(minute); err != nil {
		log.Fatalf("could not fill in minute: %v", err)
	}

	log.Printf("clicking 申請 button")
	if err := d.page.Locator(`.tm-af-applicationFormBtnArea`).GetByRole(playwright.AriaRole(`button`), playwright.LocatorGetByRoleOptions{Name: `申請`}).Click(); err != nil {
		log.Fatalf("could not click 申請 button: %v", err)
	}

	// TODO: Add wait for Dakoku history URL
}
