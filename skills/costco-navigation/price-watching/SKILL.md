---
name: costco-price-matching
description: Compare Costco prices for a user-requested product across nearby Costco warehouses, using rendered browser navigation when needed.
allowedTools:
  - webSearch
  - fetchURL
  - browserInteract
---

# Costco Price Matching

Use this skill when the user asks to compare Costco prices, check Costco
warehouse availability, or find where a Costco product is cheapest near them.

This skill is meant to work together with `browser-navigation`. If the active
skills do not include browser-navigation and the task requires opening Costco,
changing warehouse/location, searching Costco, or interacting with a rendered
page, call `request_skill_reselection` with a reason such as:

`Costco price comparison requires rendered browser navigation across nearby warehouses.`

## Required Inputs

You need:

- Product query or product name from the user.
- User location, such as city/state, ZIP code, or address.

If the user did not provide a location, ask for it before searching. Do not guess
the user's location from prior context unless it was explicitly provided in this
conversation.

## Workflow

1. Use `browserInteract` through the browser-navigation skill to open
   `https://www.costco.com/`.
2. Set or change the delivery/warehouse location using the user's provided
   location. If Costco requires a ZIP code and the user gave only a city, ask for
   a ZIP code unless the page offers a selectable city result.
3. Search Costco for the product query.
4. Open the most relevant product result. Prefer exact product matches over
   sponsored, adjacent, or bundle results.
5. Record the observed product name, URL, listed price, availability, delivery
   price when shown, and selected warehouse/location.
6. Compare nearby warehouses or locations by changing the Costco warehouse or
   location selector. Check several close stores when the site exposes them.
7. Keep each browser step narrow. Inspect returned page text and links before
   deciding the next interaction.

Do not click checkout, add-to-cart, membership purchase, payment, order
submission, legal acceptance, or irreversible account actions unless the user
explicitly asks for that exact action.

## Price Comparison Rules

- Use only prices observed from Costco pages or rendered Costco page text.
- If Costco shows online-only, delivery-only, or warehouse-only pricing, label it
  clearly.
- If prices differ by warehouse, present a table-like summary with location,
  price, availability, and URL.
- If Costco hides local warehouse prices, says login/membership is required, or
  blocks navigation, state that limitation and report any online price observed.
- Do not claim a price match policy, refund eligibility, or adjustment guarantee
  unless you observed that policy from Costco in the current run.

## Completion

Answer with:

- Best matching Costco product.
- Compared warehouse/location list.
- Cheapest observed option.
- Important limitations, such as membership/login requirements, out-of-stock
  locations, unavailable local pricing, or pages that could not be opened.
- Source URL(s) from observed Costco pages.
