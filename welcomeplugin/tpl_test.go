package welcomeplugin

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var expectedOutput = strings.ReplaceAll(`
Welcome to voldy's plugin, @sj!

We are a group of **420** people out of which **69** are online right now.
There are **9001** verified real members on this server, who meet regularly (when possible).

Users posted **20** messages today and **420** in the last week.

Please check #rules and post an introduction in #introductions to see all the other channels and start chatting in the server. 

After verification, you can head over to #roles and grab roles for notifications of events or meetups, etc.

If you have any questions, feel free to message an Admin or Mod.
Please allow up to 24 hours for us to give out permissions, we usually allow within minutes. Thank you :blush:

Reminder of the introduction template:

<CODE>
Name/Nickname: 
Age: 
Hobbies:
<CODE>
`, "<CODE>", "```")

func TestMessageRendering(t *testing.T) {
	out := renderMessage(messageVars{
		ServerName:          "voldy's plugin",
		User:                "@sj",
		TotalUsersCount:     420,
		OnlineUsersCount:    69,
		RealUsersCount:      9001, // gotta be over 9000
		MessagesToday:       20,
		MessagesLastWeek:    420,
		IntroductionChannel: "#introductions",
	})

	if diff := cmp.Diff(expectedOutput, out); diff != "" {
		t.Errorf("real output did not match expected output: %s\n", diff)
	}
}
