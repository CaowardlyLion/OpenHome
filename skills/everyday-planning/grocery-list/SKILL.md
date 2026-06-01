---
name: grocery-list
description: Read, answer questions about, create, or update a grouped grocery list using meal needs and pantry contents.
allowedTools:
  - listFiles
  - readFile
  - searchFiles
  - writeFile
---

# Grocery List

For questions about the current grocery list, read the grocery-list file and
answer from its contents. For changes, read relevant grocery-list, pantry,
preference, and meal-plan files before creating or updating a Markdown grocery
list grouped by store section. Avoid adding ingredients that are already
available in sufficient quantity when that can be inferred.

## Verification

Confirm the output file exists, is grouped by store section, and reflects the
available pantry information.
