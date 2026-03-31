[Current Execution Plan]
{{PLAN_ITEMS}}

Plan state rules:
- Treat this plan as the current execution state, not as decorative text.
- Treat the in_progress item as the current step.
- When the current step changes, or scope shifts materially, update the plan before continuing.
- Prefer changing status or adding the smallest necessary step over rewriting the whole plan.
- If repository evidence invalidates the current plan, explain that and update the plan before proceeding.
