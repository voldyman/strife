package meetupsplugin

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/pkg/errors"
)

var meetupMessages = []string{
	`What:  Private Tasting Event at O5 Tea Bar 
	Where: 2208 West 4th Ave
	When: Oct 7 (Friday) 7pm-8:30pm.
	  
	Content: Kombucha on tap, served as participants arrive. * Choice of 4-5 flavours, or samples of each. Contains very small quantities of alcohol (0.5-0.8% ABV). Then followed by a guided loose leaf tea tasting, consisting of 3-5 different teas (Menu TBD). 
	
	Price: $55 per person
	
	 Discount: Receive 20% off on any purchases of tea and select teaware, on the night of the event.
	
	  Max seating: 13  
	Edited: Kombucha info
	React with :KermitDrinking:  if you’re coming
	  @Vancouver @Events `,

	`RXS — 09/21/2022
	What: Kehlani Concert @ PNE
	
	When: Wednesday 21st (TODAY lol) 7pm
	
	
	Where: PNE
	
	If you are going, react with 🎵
	
	I’m going on my solo dolo so thought would be cool to meet up if anyone else is heading to the concert? Never gone to a concert on my own so new for me! 
	
	@Music @Vancouver`,
	`What: Drink N Draw Pumpkin Spice edition
	When: Saturday Sept 24th, 2 pm onward (early birds welcome early for extra chill crafting time) 
	Where: South East corner of Trout Lake/John Hendry Park, same location as before, exact location will be posted day of. 
	
	* no rain so we'll be at Trout Lake, but it rained last night so bring a chair if you can or a blanket if you can't. I'm bringing a tarp *
	
	Will the good weather last?  Let's see! Come hang out in the park, do some arts n crafts, drink some beverages, play some games. Bring a knitting, drawing, painting or other project, or use some of the art supplies provided! If you can, bring a camping chair or blanket to sit. Anyone who would like to bring a game, music or food is encouraged to do so! 
	
	If the weather is good and we are in the park, I'm going to bring some air dry clay for people interested in sculpting (will be a bit messy). 
	
	Details in the thread: https://discord.com/channels/707620933841453186/966501865435054150 
	
	 for arts n crafts 
	 for general park hangs (edited)
	
	14
	
	20
	
	4
	September 15, 2022
	`,
}

func TestParsing(t *testing.T) {
	for i, msg := range meetupMessages {
		t.Run(fmt.Sprint("Message", i), func(t *testing.T) {
			output, err := ParseMeetupMessage(msg)
			if err != nil {
				t.Log("error", err)
				t.Fail()
			}
			t.Log("Test", i, ": ", output)
			t.Fail()
		})
	}
}

func ParseMeetupMessage(msg string) (string, error) {
	exp, err := regexp.Compile(`When:(.*)\n`)
	if err != nil {
		return "", errors.Wrapf(err, "unable to compile 'when' detector")
	}
	matches := exp.FindAllStringSubmatch(msg, -1)
	dates := []string{}
	for _, submatch := range matches {
		if len(submatch) == 2 {
			dates = append(dates, submatch[1])
		}
	}
	return strings.Join(dates, ", "), nil
}
