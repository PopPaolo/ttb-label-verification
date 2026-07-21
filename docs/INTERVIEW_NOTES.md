## Interview Notes

### **Interview Notes: Sarah Chen, Deputy Director of Label Compliance**

"So the TTB reviews about 150,000 label applications a year. 

The actual review process is pretty straightforward. An agent pulls up an application, looks at the label artwork, and checks that what's on the label matches what's in the application. 
* Brand name matches? Check. 
* ABV is correct? Check. 
* Government warning is there? Check. 
It takes maybe 5-10 minutes per application for a simple one, longer if there are issues.

What we do is just... matching. Literally just making sure the number on the form is the same as the number on the label.

Oh, I should mention—we tried a pilot with the scanning vendor last year. Disaster. The system would take 30, 40 seconds sometimes to process a single label. Our agents just went back to doing it by eye because they could do five labels in the time it took the machine to do one. **If we can't get results back in about 5 seconds, nobody's going to use it.** We learned that the hard way.

The agents really vary in their tech comfort level. Dave's been here since the Clinton administration and still prints his emails. Meanwhile, Jenny's fresh out of college and probably could have built this tool herself. We need something **my mother could figure out**—she's 73 and just learned to video call her grandkids last year, if that gives you a benchmark. Half our team is over 50. Clean, obvious, no hunting for buttons.

One more thing that came up in our last team meeting—during peak season, we get these big importers who dump 200, 300 label applications on us at once. Right now we literally have to process them one at a time. If there was some way to **handle batch uploads**, that would be huge. Janet from our Seattle office has been asking about this for years."

### **Interview Notes: Marcus Williams, IT Systems Administrator**

"Technical landscape.

Current infrastructure is... well, it's government infrastructure, let's leave it at that. We're on Azure now after the migration in 2019. That was a whole thing—don't get me started on the FedRAMP certification process. Took 18 months just for the paperwork.

The COLA system is built on .NET, though there's been talk about modernizing it for years. We had a contractor come in last summer to do an assessment and they quoted us $4.2 million for a full rebuild. That went nowhere, obviously.

For this prototype, we're not looking to integrate with COLA directly—that's a whole different beast with its own authorization requirements. Think of this as a standalone proof-of-concept that could potentially inform future procurement decisions. If it works well, maybe we look at how to incorporate it into the workflow. But that's years away, realistically.

Security-wise, we'd need to be careful with any production deployment—there's PII considerations, document retention policies, the usual federal compliance stuff. But for a prototype? Just don't do anything crazy. We're not storing anything sensitive for this exercise.

Oh, and our network blocks outbound traffic to a lot of domains, so keep that in mind if you're thinking about cloud APIs. During the scanning vendor pilot, half their features didn't work because our firewall blocked connections to their ML endpoints. Classic."

### **Interview Notes: Dave Morrison, Senior Compliance Agent (28 years)**

"Look, I'll be honest, I've seen a lot of these 'modernization' projects come and go. Remember the automated phone system they put in back in 2008? Supposed to reduce call volume. We ended up with more calls because nobody could figure out how to navigate it.

The thing about label review is there's nuance. You can't just pattern match everything. Like, I had one last week where the brand name was 'STONE'S THROW' on the label but 'Stone's Throw' in the application. Technically a mismatch? Sure. But it's obviously the same thing. You need judgment.

That said, I'm not against new tools. If something can help me get through my queue faster, great. Just don't make my life harder in the process. I spend enough time fighting with COLA as it is."

### **Interview Notes: Jenny Park, Junior Compliance Agent (8 months)**

"I'm so excited you're working on this! When I started here, I was kind of shocked at how manual everything is. Like, I literally have a printed checklist on my desk that I go through for every label. Brand name—check with my eyes. ABV—check with my eyes. Warning statement—check with my eyes. It's 2024!

The one thing I'd say is the warning statement check is actually trickier than it sounds. It has to be **exact**. Like, word-for-word, and the 'GOVERNMENT WARNING:' part has to be in all caps and bold. Sarah probably mentioned this but people try to get creative with the warning all the time. Smaller font, different wording, burying it in tiny text. I caught one last month where they used 'Government Warning' in title case instead of all caps. Rejected.

Also—and this is maybe out of scope for a prototype—but it would be amazing if the tool could handle images that aren't perfectly shot. I've seen labels that are photographed at weird angles, or the lighting is bad, or there's glare on the bottle. Right now if an agent can't read the label they just reject it and ask for a better image. But if AI could handle some of that..."