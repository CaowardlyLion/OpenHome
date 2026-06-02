---
name: web-search
description: Research fresh internet information by searching the web and opening relevant result pages before answering.
allowedTools:
  - webSearch
  - fetchURL
  - browserInteract
---

# Web Search

Use `webSearch` to discover relevant pages. Treat search titles and snippets as
leads, not as sufficient evidence for the final answer.

Open the most relevant result pages with `fetchURL` before answering. Read enough
page content to answer the user's actual question. Prefer primary sources when
available. For time-sensitive facts, check that the source is current and
cross-check another relevant source when practical.

When the user asks for sources, use the most specific observed URL available.
For article headlines, prefer the matching article URL from `fetchURL` links
over a publication homepage or category page.

If a search result page cannot be fetched, try another relevant result. If
permission is denied or no usable source can be opened, explain that limitation
instead of presenting search snippets as verified facts.

If `fetchURL` reports that static fetching was blocked and recommends
`browser-navigation`, call `request_skill_reselection` so the librarian can
select that skill. If a relevant page otherwise requires rendered JavaScript or
browser interaction, use `browserInteract`. Do not click buttons that submit
forms, place orders, send messages, or otherwise create side effects unless the
user explicitly requested that action and approves the browser interaction.

If the user asks for a specific time-frame, prioritize sources from that period.
For example, if the user asks "What are the latest developments in X?", prioritize 
sources from the past few days or weeks. If the user asks "What was the state of Y
in 2010?", prioritize sources from around that year. Requests for news should be 
answered with recent sources, and questions regarding things "today" should 
only be answered with sources from the current day.

## Completion

Answer from the opened pages. Include useful source URLs and distinguish any
remaining uncertainty.
