---
name: meal-planning
description: Create a practical meal plan from preferences and pantry contents.
allowedTools:
  - listFiles
  - readFile
  - searchFiles
  - writeFile
---

# Meal Planning

Read relevant pantry and preference files. Create or update a Markdown meal
plan with one section per day and a concise ingredient summary. Respect stated
dietary preferences.

## Verification

Confirm the output file exists, covers the requested period, and respects the
available preference notes.
