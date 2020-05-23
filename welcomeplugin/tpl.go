package welcomeplugin

import (
	"bytes"
	"strings"
	"text/template"
)

const templateText = `
Welcome to {{.ServerName}}, {{.User}}!

We are a group of **{{.TotalUsersCount}}** people out of which **{{.OnlineUsersCount}}** are online right now.
There are **{{.RealUsersCount}}** verified real members on this server, who meet regularly (when possible).

Users posted **{{.MessagesToday}}** messages today and **{{.MessagesLastWeek}}** in the last week.

Please check #rules and post an introduction in #introductions to see all the other channels and start chatting in the server. 

After verification, you can head over to #roles and grab roles for notifications of events or meetups, etc.

If you have any questions, feel free to message an Admin or Mod.
Please allow up to 24 hours for us to give out permissions, we usually allow within minutes. Thank you :blush:

Reminder of the introductions template: 
<CODE>
Name/Nickname: 
Age: 
Hobbies:
<CODE>
`

var messageTemplate = template.Must(template.New("message").Parse(formattedTemplateText()))

func formattedTemplateText() string {
	return strings.ReplaceAll(templateText, "<CODE>", "```")
}

type messageVars struct {
	ServerName       string
	User             string
	TotalUsersCount  int
	OnlineUsersCount int
	RealUsersCount   int
	MessagesToday    int
	MessagesLastWeek int
}

func renderMessage(vars messageVars) string {
	var buf bytes.Buffer
	messageTemplate.Execute(&buf, vars)

	return buf.String()
}
