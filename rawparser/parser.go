package rawparser

import (
	"regexp"
)

const (
	// NOTE: adapted from https://github.com/youyo/postfix-log-parser.git
	possibleMonths                    = `Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec`
	timeRawSmtpSentStatusRegexpFormat = `(?P<Time>(?P<Month>(` + possibleMonths + `))\s\s?(?P<Day>[0-9]{1,2}) (?P<Hour>[0-9]{2}):(?P<Minute>[0-9]{2}):(?P<Second>[0-9]{2}))`
	hostRawSmtpSentStatusRegexpFormat = `(?P<Host>[0-9A-Za-z\.]+)`
	// TODO: the process name can have more slash separated components, such as: postfix/submission/smtpd
	processRawSmtpSentStatusRegexpFormat = `(postfix(-[^/]+)?/(?P<Process>[a-z]+)\[[0-9]{1,5}\])`
	queueIdRawSmtpSentStatusRegexpFormat = `(?P<Queue>[0-9A-F]+)`

	headerRegexpFormat = `^` + timeRawSmtpSentStatusRegexpFormat + ` ` + hostRawSmtpSentStatusRegexpFormat + ` ` + processRawSmtpSentStatusRegexpFormat + `: `

	anythingExceptCommaRegexpFormat = `[^,]+`

	// Relay name might be absent, having only "none"
	relayComponentsRegexpFormat = `((?P<RelayName>[^\,[]+)` + `\[(?P<RelayIp>[^\],]+)\]` + `:` + `(?P<RelayPort>[\d]+)|none)`

	messageSentWithStatusRawSmtpSentStatusRegexpFormat = `(?P<MessageSentWithStatus>` +
		`to=<(?P<RecipientLocalPart>[^@]+)@(?P<RecipientDomainPart>[^>]+)>` + `, ` +
		`relay=` + relayComponentsRegexpFormat + `, ` +
		`delay=(?P<Delay>` + anythingExceptCommaRegexpFormat + `)` + `, ` +
		`delays=(?P<Delays>(?P<Delays0>[^/]+)/(?P<Delays1>[^/]+)/(?P<Delays2>[^/]+)/(?P<Delays3>[^/]+))` + `, ` +
		`dsn=(?P<Dsn>` + anythingExceptCommaRegexpFormat + `)` + `, ` +
		`status=(?P<Status>(deferred|bounced|sent))` + ` ` +
		`(?P<ExtraMessage>.*)` +
		`)`

	possibleSmtpPayloadsFormat = messageSentWithStatusRawSmtpSentStatusRegexpFormat

	smtpPayloadsRegexpFormat = `^` + queueIdRawSmtpSentStatusRegexpFormat + `: ` +
		`(` + possibleSmtpPayloadsFormat + `)$`
)

type RawHeader struct {
	Time    []byte
	Month   []byte
	Day     []byte
	Hour    []byte
	Minute  []byte
	Second  []byte
	Host    []byte
	Process []byte
}

type RawPayload interface {
	isRawPayload()
}

type RawRecord struct {
	Header  RawHeader
	Payload RawPayload
}

type RawSmtpSentStatus struct {
	Queue               []byte
	RecipientLocalPart  []byte
	RecipientDomainPart []byte
	RelayName           []byte
	RelayIp             []byte
	RelayPort           []byte
	Delay               []byte
	Delays              [5][]byte
	Dsn                 []byte
	Status              []byte
	ExtraMessage        []byte
}

func (RawSmtpSentStatus) isRawPayload() {
}

func indexForGroup(r *regexp.Regexp, name string) int {
	e := r.SubexpNames()
	for i, v := range e {
		if v == name {
			return i
		}
	}

	panic("Wrong Group Name: " + name + "!")
}

var (
	possiblePayloadsRegexp *regexp.Regexp
	headerRegex            *regexp.Regexp

	timeIndex    int
	monthIndex   int
	dayIndex     int
	hourIndex    int
	minuteIndex  int
	secondIndex  int
	hostIndex    int
	processIndex int

	messageSentWithStatusIndex   int
	smtpQueueIndex               int
	smtpRecipientLocalPartIndex  int
	smtpRecipientDomainPartIndex int
	smtpRelayNameIndex           int
	smtpRelayIpIndex             int
	smtpRelayPortIndex           int
	smtpDelayIndex               int
	smtpDelaysIndex              int
	smtpDelays0Index             int
	smtpDelays1Index             int
	smtpDelays2Index             int
	smtpDelays3Index             int
	smtpDsnIndex                 int
	smtpStatusIndex              int
	smtpExtraMessageIndex        int
)

func init() {
	possiblePayloadsRegexp = regexp.MustCompile(smtpPayloadsRegexpFormat)
	headerRegex = regexp.MustCompile(headerRegexpFormat)

	timeIndex = indexForGroup(headerRegex, "Time")
	monthIndex = indexForGroup(headerRegex, "Month")
	dayIndex = indexForGroup(headerRegex, "Day")
	hourIndex = indexForGroup(headerRegex, "Hour")
	minuteIndex = indexForGroup(headerRegex, "Minute")
	secondIndex = indexForGroup(headerRegex, "Second")
	hostIndex = indexForGroup(headerRegex, "Host")
	processIndex = indexForGroup(headerRegex, "Process")

	messageSentWithStatusIndex = indexForGroup(possiblePayloadsRegexp, "MessageSentWithStatus")
	smtpQueueIndex = indexForGroup(possiblePayloadsRegexp, "Queue")
	smtpRecipientLocalPartIndex = indexForGroup(possiblePayloadsRegexp, "RecipientLocalPart")
	smtpRecipientDomainPartIndex = indexForGroup(possiblePayloadsRegexp, "RecipientDomainPart")
	smtpRelayNameIndex = indexForGroup(possiblePayloadsRegexp, "RelayName")
	smtpRelayIpIndex = indexForGroup(possiblePayloadsRegexp, "RelayIp")
	smtpRelayPortIndex = indexForGroup(possiblePayloadsRegexp, "RelayPort")
	smtpDelayIndex = indexForGroup(possiblePayloadsRegexp, "Delay")
	smtpDelaysIndex = indexForGroup(possiblePayloadsRegexp, "Delays")
	smtpDelays0Index = indexForGroup(possiblePayloadsRegexp, "Delays0")
	smtpDelays1Index = indexForGroup(possiblePayloadsRegexp, "Delays1")
	smtpDelays2Index = indexForGroup(possiblePayloadsRegexp, "Delays2")
	smtpDelays3Index = indexForGroup(possiblePayloadsRegexp, "Delays3")
	smtpDsnIndex = indexForGroup(possiblePayloadsRegexp, "Dsn")
	smtpStatusIndex = indexForGroup(possiblePayloadsRegexp, "Status")
	smtpExtraMessageIndex = indexForGroup(possiblePayloadsRegexp, "ExtraMessage")
}

func ParseLogLine(logLine []byte) (RawRecord, error) {
	headerMatches := headerRegex.FindSubmatch(logLine)

	if len(headerMatches) == 0 {
		return RawRecord{}, InvalidHeaderLineError
	}

	header := RawHeader{
		Time:    headerMatches[timeIndex],
		Month:   headerMatches[monthIndex],
		Day:     headerMatches[dayIndex],
		Hour:    headerMatches[hourIndex],
		Minute:  headerMatches[minuteIndex],
		Second:  headerMatches[secondIndex],
		Host:    headerMatches[hostIndex],
		Process: headerMatches[processIndex],
	}

	linePayload := logLine[len(headerMatches[0]):]

	// NOTE: hopefully the compiler will not heap allocate a string here,
	// but use the slice content directly
	switch string(headerMatches[processIndex]) {
	case "smtp":
		return parseSmtpPayload(header, linePayload)
	default:
		// TODO: implement support for other processes
		return RawRecord{}, UnsupportedLogLineError
	}
}

func parseSmtpPayload(header RawHeader, linePayload []byte) (RawRecord, error) {
	payloadMatches := possiblePayloadsRegexp.FindSubmatch(linePayload)

	if len(payloadMatches) == 0 {
		return RawRecord{}, UnsupportedLogLineError
	}

	if len(payloadMatches[messageSentWithStatusIndex]) == 0 {
		// TODO: implement other stuff done by the "smtp" process
		return RawRecord{}, UnsupportedLogLineError
	}

	s := RawSmtpSentStatus{
		Queue:               payloadMatches[smtpQueueIndex],
		RecipientLocalPart:  payloadMatches[smtpRecipientLocalPartIndex],
		RecipientDomainPart: payloadMatches[smtpRecipientDomainPartIndex],
		RelayName:           payloadMatches[smtpRelayNameIndex],
		RelayIp:             payloadMatches[smtpRelayIpIndex],
		RelayPort:           payloadMatches[smtpRelayPortIndex],
		Delay:               payloadMatches[smtpDelayIndex],
		Delays: [5][]byte{payloadMatches[smtpDelaysIndex],
			payloadMatches[smtpDelays0Index],
			payloadMatches[smtpDelays1Index],
			payloadMatches[smtpDelays2Index],
			payloadMatches[smtpDelays3Index]},
		Dsn:          payloadMatches[smtpDsnIndex],
		Status:       payloadMatches[smtpStatusIndex],
		ExtraMessage: payloadMatches[smtpExtraMessageIndex],
	}

	return RawRecord{header, s}, nil
}
