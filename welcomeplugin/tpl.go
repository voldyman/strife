package welcomeplugin

import (
	"bytes"
	"text/template"
)

const templateText = `
Welcome to {{.ServerName}}, {{.User}}!

We are a group of {{.TotalUsersCount}} people out of which {{.OnlineUsersCount}} are online right now.
There are {{.RealUsersCount}} verified real members on this server, who meet regularly (when possible).

Please check #rules and post an introduction in #introductions to see all the other channels and start chatting in the server. 

After verification, you can head over to #roles and grab roles for notifications of events or meetups, etc.

If you have any questions, feel free to message an Admin or Mod.
Please allow up to 24 hours for us to give out permissions, we usually allow within minutes. Thank you :blush:

Reminder of the introductions template: 
Name/Nickname: 
Age: 
Hobbies: 
Looking for:
`

var messageTemplate = template.Must(template.New("message").Parse(templateText))

type messageVars struct {
	ServerName       string
	User             string
	TotalUsersCount  int
	OnlineUsersCount int
	RealUsersCount   int
}

func renderMessage(vars messageVars) string {
	var buf bytes.Buffer
	messageTemplate.Execute(&buf, vars)

	return buf.String()
}
