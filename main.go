package main

import (
	"encoding/json"
	"fmt"
	"github.com/akrylysov/algnhsa"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/guregu/dynamo"
	"github.com/line/clova-cek-sdk-go/cek"
	"github.com/line/line-bot-sdk-go/linebot"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Record struct {
	Day      int       `dynamo:"day"`
	PunchIn  time.Time `dynamo:"punch_in"`
	PunchOut time.Time `dynamo:"punch_out"`
}

type TimeCardData struct {
	UserIdYearMonth string   `dynamo:"userid-year-month"`
	Records         []Record `dynamo:"records"`
}

func sessionEndSpeech(speech string) *cek.ResponseMessage {
	return cek.NewResponseBuilder().
		OutputSpeech(
			cek.NewOutputSpeechBuilder().
				AddSpeechText(speech, cek.SpeechInfoLangJA).Build()).
		ShouldEndSession(true).
		Build()
}

func sessionContinueSpeech(speech string, session *cek.Session) *cek.ResponseMessage {
	return cek.NewResponseBuilder().
		OutputSpeech(
			cek.NewOutputSpeechBuilder().
				AddSpeechText(speech, cek.SpeechInfoLangJA).Build()).
		SessionAttributes(session.SessionAttributes).
		ShouldEndSession(false).
		Build()
}

func handleIntentRequest(req *cek.IntentRequest, session *cek.Session) *cek.ResponseMessage {
	switch req.Intent.Name {
	case "PunchInIntent":
		return punchInIntent(req, session)
	case "PunchOutIntent":
		return punchOutIntent(req, session)
	case "GetDurationIntent":
		return getDurationIntent(req, session)
	case "GetThisMonthIntent":
		return getThisMonthIntent(req, session)
	case "GetLastMonthIntent":
		return getLastMonthIntent(req, session)
	default:
		return sessionContinueSpeech("すみません、もう一度言ってください。", session)
	}
}

func punchInIntent(req *cek.IntentRequest, session *cek.Session) *cek.ResponseMessage {
	t := time.Now()

	awsSession, err := awssession.NewSession()
	if err != nil {
		log.Printf("aws session failed =%+v", err)
		return sessionEndSpeech("処理異常が発生しました。")
	}
	table := dynamo.New(awsSession).Table("timecard-clova")

	var timeCardData TimeCardData

	err = table.Get("userid-year-month", session.User.UserID+"-"+t.Format("2006-01")).One(&timeCardData)

	if err != nil && err != dynamo.ErrNotFound {
		log.Printf("dynamo get failed =%+v", err)
		return sessionEndSpeech("処理異常が発生しました。")
	}

	if err == dynamo.ErrNotFound {
		timeCardData.UserIdYearMonth = session.User.UserID + "-" + t.Format("2006-01")
	}

	for _, record := range timeCardData.Records {
		if record.Day == t.Day() {
			return sessionEndSpeech("今日の出勤はすでに記録されています。")
		}
	}

	timeCardData.Records = append(timeCardData.Records, Record{Day: t.Day(), PunchIn: t})

	err = table.Put(timeCardData).Run()

	if err != nil {
		log.Printf("dynamo put failed =%+v", err)
		return sessionEndSpeech("処理異常が発生しました。")
	}

	const layout = "2006-01-02 15:04:05"
	sendMessage(session.User.UserID, "出勤 "+t.Format(layout))
	return sessionEndSpeech("出勤記録しました。")
}

func punchOutIntent(req *cek.IntentRequest, session *cek.Session) *cek.ResponseMessage {
	t := time.Now()

	awsSession, err := awssession.NewSession()
	if err != nil {
		log.Printf("aws session failed =%+v", err)
		return sessionEndSpeech("処理異常が発生しました。")
	}
	table := dynamo.New(awsSession).Table("timecard-clova")

	var timeCardData TimeCardData

	err = table.Get("userid-year-month", session.User.UserID+"-"+t.Format("2006-01")).One(&timeCardData)

	if err != nil && err != dynamo.ErrNotFound {
		log.Printf("dynamo get failed =%+v", err)
		return sessionEndSpeech("処理異常が発生しました。")
	}

	if err == dynamo.ErrNotFound {
		return sessionEndSpeech("今日の出勤が記録されていません。")
	}

	isPunchedIn := false

	var duration time.Duration

	for i, record := range timeCardData.Records {
		if record.Day == t.Day() {
			timeCardData.Records[i].PunchOut = t
			duration = t.Sub(record.PunchIn)
			isPunchedIn = true
		}
	}

	if !isPunchedIn {
		return sessionEndSpeech("今日の出勤が記録されていません。")
	}

	err = table.Put(timeCardData).Run()

	if err != nil {
		log.Printf("dynamo put failed =%+v", err)
		return sessionEndSpeech("処理異常が発生しました。")
	}

	minutesTotal := int(duration.Minutes())
	hours := strconv.Itoa(minutesTotal / 60)
	minutes := strconv.Itoa(minutesTotal % 60)

	const layout = "2006-01-02 15:04:05"
	sendMessage(session.User.UserID, "退勤 "+t.Format(layout)+"\n勤務時間"+hours+"時間"+minutes+"分")
	return sessionEndSpeech("退勤記録しました。 勤務時間は" + hours + "時間" + minutes + "分でした。")
}

func getDurationIntent(req *cek.IntentRequest, session *cek.Session) *cek.ResponseMessage {
	t := time.Now()

	awsSession, err := awssession.NewSession()
	if err != nil {
		log.Printf("aws session failed =%+v", err)
		return sessionEndSpeech("処理異常が発生しました。")
	}
	table := dynamo.New(awsSession).Table("timecard-clova")

	var timeCardData TimeCardData

	err = table.Get("userid-year-month", session.User.UserID+"-"+t.Format("2006-01")).One(&timeCardData)

	if err != nil && err != dynamo.ErrNotFound {
		log.Printf("dynamo get failed =%+v", err)
		return sessionEndSpeech("処理異常が発生しました。")
	}

	if err == dynamo.ErrNotFound {
		return sessionEndSpeech("今日の出勤が記録されていません。")
	}

	isPunchedIn := false

	var duration time.Duration

	for i, record := range timeCardData.Records {
		if record.Day == t.Day() {
			timeCardData.Records[i].PunchOut = t
			duration = t.Sub(record.PunchIn)
			isPunchedIn = true
		}
	}

	if !isPunchedIn {
		return sessionEndSpeech("今日の出勤が記録されていません。")
	}

	minutesTotal := int(duration.Minutes())
	hours := strconv.Itoa(minutesTotal / 60)
	minutes := strconv.Itoa(minutesTotal % 60)

	return sessionEndSpeech("今日の現在までの勤務時間は" + hours + "時間" + minutes + "分です。")
}

func getThisMonthIntent(req *cek.IntentRequest, session *cek.Session) *cek.ResponseMessage {
	t := time.Now()

	awsSession, err := awssession.NewSession()
	if err != nil {
		log.Printf("aws session failed =%+v", err)
		return sessionEndSpeech("処理異常が発生しました。")
	}
	table := dynamo.New(awsSession).Table("timecard-clova")

	var timeCardData TimeCardData

	err = table.Get("userid-year-month", session.User.UserID+"-"+t.Format("2006-01")).One(&timeCardData)

	if err != nil && err != dynamo.ErrNotFound {
		log.Printf("dynamo get failed =%+v", err)
		return sessionEndSpeech("処理異常が発生しました。")
	}

	if err == dynamo.ErrNotFound {
		return sessionEndSpeech("今月の出勤は記録されていません。")
	}

	durationMinutesTotal := 0
	workingDays := 0
	var nullTime time.Time

	lineString := t.Format("2006年01月")

	for i, record := range timeCardData.Records {
		lineString += "\n" + strconv.Itoa(record.Day) + "日 出社: " + fmt.Sprintf("%02d", record.PunchIn.Hour()) + ":" + fmt.Sprintf("%02d", record.PunchIn.Minute())
		if record.PunchOut != nullTime {
			lineString += " 退社: " + fmt.Sprintf("%02d", record.PunchOut.Hour()) + ":" + fmt.Sprintf("%02d", record.PunchOut.Minute())
		}
		if record.PunchOut != nullTime && record.Day < t.Day() {
			timeCardData.Records[i].PunchOut = t
			durationMinutesTotal += int(t.Sub(record.PunchIn).Minutes())
			workingDays += 1
		}
	}

	if workingDays == 0 {
		sendMessage(session.User.UserID, lineString)
		return sessionEndSpeech("今月の出勤記録をLINEにお送りしました。")
	}

	hours := strconv.Itoa(durationMinutesTotal / 60)
	minutes := strconv.Itoa(durationMinutesTotal % 60)

	hoursPerDay := strconv.Itoa(durationMinutesTotal / workingDays / 60)
	minutesPerDay := strconv.Itoa((durationMinutesTotal / workingDays) % 60)

	lineString += fmt.Sprintf("\n出勤日数(昨日まで): %d日", workingDays)
	lineString += "総勤務時間:" + hours + "時間" + minutes + "分\n1日平均: " + hoursPerDay + "時間" + minutesPerDay + "分"

	sendMessage(session.User.UserID, lineString)

	return sessionEndSpeech("今月の昨日までの総勤務時間は" + hours + "時間" + minutes + "分です。1日平均は" + hoursPerDay + "時間" + minutesPerDay + "分です。詳細な記録はLINEにお送りしました。")
}

func getLastMonthIntent(req *cek.IntentRequest, session *cek.Session) *cek.ResponseMessage {
	t := time.Now()

	year := t.Year()
	month := t.Month()

	if month == 1 {
		year -= 1
		month = 12
	} else {
		month -= 1
	}

	awsSession, err := awssession.NewSession()
	if err != nil {
		log.Printf("aws session failed =%+v", err)
		return sessionEndSpeech("処理異常が発生しました。")
	}
	table := dynamo.New(awsSession).Table("timecard-clova")

	var timeCardData TimeCardData

	err = table.Get("userid-year-month", fmt.Sprintf("%s-%04d-%02d", session.User.UserID, year, month)).One(&timeCardData)

	if err != nil && err != dynamo.ErrNotFound {
		log.Printf("dynamo get failed =%+v", err)
		return sessionEndSpeech("処理異常が発生しました。")
	}

	if err == dynamo.ErrNotFound {
		return sessionEndSpeech("今月の出勤は記録されていません。")
	}

	durationMinutesTotal := 0
	workingDays := 0
	var nullTime time.Time

	lineString := fmt.Sprintf("%04d年%02d月", year, month)

	for i, record := range timeCardData.Records {
		lineString += "\n" + strconv.Itoa(record.Day) + "日 出社: " + fmt.Sprintf("%02d", record.PunchIn.Hour()) + ":" + fmt.Sprintf("%02d", record.PunchIn.Minute())
		if record.PunchOut != nullTime {
			lineString += " 退社: " + fmt.Sprintf("%02d", record.PunchOut.Hour()) + ":" + fmt.Sprintf("%02d", record.PunchOut.Minute())
		}
		if record.PunchOut != nullTime {
			timeCardData.Records[i].PunchOut = t
			durationMinutesTotal += int(t.Sub(record.PunchIn).Minutes())
			workingDays += 1
		}
	}

	if workingDays == 0 {
		sendMessage(session.User.UserID, lineString)
		return sessionEndSpeech("先月の出勤記録をLINEにお送りしました。")
	}

	hours := strconv.Itoa(durationMinutesTotal / 60)
	minutes := strconv.Itoa(durationMinutesTotal % 60)

	hoursPerDay := strconv.Itoa(durationMinutesTotal / workingDays / 60)
	minutesPerDay := strconv.Itoa((durationMinutesTotal / workingDays) % 60)

	lineString += fmt.Sprintf("\n出勤日数: %d日", workingDays)
	lineString += "総勤務時間:" + hours + "時間" + minutes + "分\n1日平均: " + hoursPerDay + "時間" + minutesPerDay + "分"

	sendMessage(session.User.UserID, lineString)

	return sessionEndSpeech("先月の総勤務時間は" + hours + "時間" + minutes + "分です。1日平均は" + hoursPerDay + "時間" + minutesPerDay + "分です。詳細な記録はLINEにお送りしました。")
}

func sendMessage(id, text string) {
	client := &http.Client{}
	bot, err := linebot.New(os.Getenv("CHANNEL_SECRET"), os.Getenv("CHANNEL_ACCESS_TOKEN"), linebot.WithHTTPClient(client))
	if err != nil {
		log.Printf("LINE bot client initialization error.")
		return
	}
	msg := linebot.NewTextMessage(text)
	_, err = bot.PushMessage(id, msg).Do()
	if err != nil {
		log.Printf("message pushing failed. err=%q", err)
		return
	}
	log.Printf("message pushing succeeded.")
}

func TimeCard(w http.ResponseWriter, r *http.Request) {

	ext := cek.NewExtension(os.Getenv("applicationId"))
	requestMessage, err := ext.ParseRequest(r)
	if err != nil {
		log.Printf("invalid request. err=%+v", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	var response *cek.ResponseMessage
	switch request := requestMessage.Request.(type) {
	case *cek.IntentRequest:
		response = handleIntentRequest(request, requestMessage.Session)
	case *cek.LaunchRequest:
		response = sessionContinueSpeech("タイムカードです。出社記録、退社記録、経過時間、今月の記録、先月の記録から選んでください。", requestMessage.Session)
	case *cek.SessionEndedRequest:
		response = sessionEndSpeech("タイムカード終了します。")
	default:
		response = sessionContinueSpeech("すみません、もう一度言ってください。", requestMessage.Session)
	}
	if response != nil {
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
}

func main() {
	http.HandleFunc("/", TimeCard)
	algnhsa.ListenAndServe(http.DefaultServeMux, nil)
}
