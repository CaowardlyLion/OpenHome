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

If a search result page cannot be fetched, try another relevant result. If
permission is denied or no usable source can be opened, explain that limitation
instead of presenting search snippets as verified facts.

If a relevant page requires rendered JavaScript or browser interaction, use
`browserInteract`. Do not click buttons that submit forms, place orders, send
messages, or otherwise create side effects unless the user explicitly requested
that action and approves the browser interaction.

## Completion

Answer from the opened pages. Include useful source URLs and distinguish any
remaining uncertainty.
