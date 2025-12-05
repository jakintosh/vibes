# Event Scheduler UX Improvement Recommendations

> This document captures proposed UX enhancements based on the real-world workflow for managing event space rentals. These recommendations address key pain points in the current request → approval → confirmation → execution flow.

## Context & Goals

### Primary Users
- **Administrators**: Manage the event pipeline—reviewing requests, approving events, tracking payments, coordinating staffing
- **Finance Person**: Needs visibility into expected payments and what money is for
- **External Requesters**: Submit requests through the public form (not covered here)

### Core Workflow
1. External person submits a request via the form
2. Admins discuss requests in group meetings and decide to approve/deny
3. Approved events get an offer sent to the requester (date + price)
4. Requester accepts, negotiates, declines, or ghosts
5. Confirmed events need staffing (opener/closer assigned)
6. Confirmed events need payment collection
7. Events may be modified or canceled after confirmation

### Key Pain Points Addressed
- No good way to discuss requests during meetings
- No tracking of offer/response status after acceptance
- No visibility into "who is working" each event
- Payment tracking spread across views, hard for finance handoff
- No edit capability for post-acceptance changes

---

## Recommendations by Category

---

## 1. Request Review Flow Improvements

### Problem
When requests come in, admins need to quickly assess them during in-person group meetings. Currently, they must navigate through multiple pages and mentally track discussions.

### Recommendations

#### 1.1 Internal Notes Field
**Priority: HIGH** | **Effort: LOW**

Add a text field for internal notes on each event. This allows admins to:
- Document discussion points during meetings ("requester is a first-timer, give extra setup instructions")
- Record special arrangements ("they'll bring their own AV equipment")
- Note concerns or context ("booked this same slot last year, no issues")

**Implementation:**
- Add `internal_notes TEXT` column to events table
- Add editable text area to event detail page
- Notes should NOT be visible on any public-facing views

#### 1.2 Meeting Mode / Presentation View
**Priority: MEDIUM** | **Effort: MEDIUM**

A simplified view optimized for projecting during group meetings:
- Large, readable cards for each pending request
- Shows: title, requester name, requested dates with conflict status, days waiting
- Arrow key navigation between requests
- Quick action buttons: Approve, Deny, Hold

#### 1.3 Quick Comparison Calendar
**Priority: MEDIUM** | **Effort: MEDIUM**

When viewing a request with multiple date options, embed a mini week-view calendar showing conflicts inline, so admins don't need to open a new tab to check the calendar.

#### 1.4 "Hold" Status
**Priority: LOW** | **Effort: LOW**

Add a `StatusHeld` state for requests that are actively being discussed but not yet decided. This prevents them from falling through the cracks and distinguishes "we're thinking about it" from "we haven't seen it yet."

---

## 2. Acceptance & Offering Flow

### Problem
After approving a request, someone needs to contact the requester with the offer details. Currently, admins manually compose these communications.

### Recommendations

#### 2.1 Email Template Generation
**Priority: MEDIUM** | **Effort: LOW**

After accepting a request, display a pre-filled email template that can be copied:

```
Subject: Your Event Request - [Event Title] on [Date]

Hi [Contact Name],

Great news! We're pleased to offer you [Event Space Name] for your event:

Event: [Title]
Date: [Formatted Date]
Time: [Start Time] - [End Time]

Pricing:
- Total: $[Proposed Cost]
- Deposit: $[Deposit Amount] (due to confirm)
- Balance: $[Remaining] (due by [event date])

Please reply to confirm your booking. We'll secure your spot once we receive the deposit.

Looking forward to your event!
[Signature]
```

#### 2.2 Pricing Presets
**Priority: LOW** | **Effort: LOW**

Add quick-select buttons for common pricing tiers:
- "Standard Rate" → auto-fills $X
- "Non-profit Rate" → auto-fills $Y  
- "Full Day Rate" → auto-fills $Z

These values could be configurable in settings.

---

## 3. Response Tracking After Offering

### Problem
After sending an offer, requesters may accept, negotiate, decline, or never respond. There's currently no way to distinguish between "we offered but haven't heard back" vs "they confirmed."

### Recommendations

#### 3.1 Acceptance Sub-States
**Priority: HIGH** | **Effort: MEDIUM**

Split the "Accepted" state into more granular states:

| State | Meaning |
|-------|---------|
| `Offered` | Admin approved, offer sent to requester |
| `AwaitingDeposit` | Requester said yes, waiting for deposit |
| `Confirmed` | Deposit received (or waived), event is locked in |

This allows the admin dashboard to show:
- "Offers awaiting response" (Offered status)
- "Awaiting deposits" (AwaitingDeposit status)

#### 3.2 Days Since Offer
**Priority: MEDIUM** | **Effort: LOW**

Show "X days since offer" badge on events in Offered status. Helps identify stale offers that need follow-up.

#### 3.3 Follow-Up Actions
**Priority: LOW** | **Effort: LOW**

Quick action buttons:
- "Mark as No Response" → moves to a soft-declined state
- "Resend Offer" → regenerates the email template with current date

---

## 4. Event Day Staffing

### Problem
For each confirmed event, someone needs to let the renters in (opener) and lock up after (closer). There's no way to track this assignment, and it's easy to have events fall through the cracks.

### Recommendations

#### 4.1 Staffing Assignments on Event Detail Page
**Priority: HIGH** | **Effort: MEDIUM**

Add a "Staffing" section on the event detail page:
- **Opener**: Text field (or user selector) for who opens
- **Closer**: Text field (or user selector) for who closes
- **Notes**: Special instructions for staff

#### 4.2 "Needs Staffing" Indicators
**Priority: HIGH** | **Effort: LOW**

- Badge on event detail page when opener/closer not assigned
- Icon overlay on calendar views (month/week) for unstaffed events
- Filter on admin dashboard: "Needs Staffing" section

#### 4.3 Admin Dashboard Staffing Section
**Priority: MEDIUM** | **Effort: MEDIUM**

New section on admin dashboard showing upcoming confirmed events sorted by date, with staffing status:
- ✓ Fully staffed (opener + closer assigned)
- ⚠ Partially staffed (one assigned)
- ✗ Needs staffing (neither assigned)

---

## 5. Payment Tracking & Finance Handoff

### Problem
The person managing finances is different from the person managing correspondence. They need a clear view of expected payments without digging through event details.

### Recommendations

#### 5.1 Payments Dashboard Page
**Priority: HIGH** | **Effort: MEDIUM**

New page at `/payments` or `/treasury`:
- List of all events with outstanding balances
- Columns: Event Title, Date, Total Due, Received, Balance, Status
- Sortable by event date, balance amount
- Quick inline "Record Payment" button
- Filter: Show only events with balance due

#### 5.2 Payment Ledger on Event Detail
**Priority: MEDIUM** | **Effort: MEDIUM**

Instead of just showing totals, show a timeline:
- "Proposed: $X on [date]"
- "Deposit received: $Y on [date]"
- "Balance received: $Z on [date]"

#### 5.3 Expected Payments View
**Priority: MEDIUM** | **Effort: LOW**

Filtered views:
- "Deposits expected this week" (AwaitingDeposit events)
- "Final payments due before upcoming events"

#### 5.4 Export Capability
**Priority: LOW** | **Effort: LOW**

- Copy-to-clipboard formatted text of payment expectations
- Or CSV export for spreadsheet import

---

## 6. Post-Confirmation Modifications

### Problem
People cancel, change dates, or modify details after confirmation. Admins need to update the record and track what changed.

### Recommendations

#### 6.1 Edit Event Capability
**Priority: HIGH** | **Effort: MEDIUM**

Allow admins to modify after acceptance:
- Title, description
- Date/time (with conflict check)
- Payment terms (proposed cost, deposit)
- Contact info

Should require confirmation for date changes that would cause conflicts.

#### 6.2 Activity/Audit Log
**Priority: MEDIUM** | **Effort: MEDIUM**

Show a timeline of all changes on the event detail page:
- "Created on [date]"
- "Accepted on [date] for [date range]"
- "Confirmed on [date]"
- "Date changed from X to Y on [date]"
- "Payment of $X recorded on [date]"

#### 6.3 Cancellation Flow with Deposit Handling
**Priority: MEDIUM** | **Effort: LOW**

When canceling a confirmed event:
- If deposit was collected, prompt: "Refund deposit?" with options
- Track whether deposit was refunded or retained
- Update payment status accordingly

---

## 7. Dashboard & Navigation Improvements

### Recommendations

#### 7.1 Pending Actions Widget
**Priority: MEDIUM** | **Effort: MEDIUM**

Sidebar or header widget showing counts:
- 🔔 3 requests awaiting review
- 📤 2 offers awaiting response
- 👤 4 events needing staffing
- 💰 2 payments due

Clicking each navigates to the appropriate filtered view.

#### 7.2 "This Week" Quick View
**Priority: LOW** | **Effort: MEDIUM**

Dashboard section showing just this week's events with:
- Day-by-day breakdown
- Staffing status at a glance
- Payment status
- Quick actions

#### 7.3 View Filters
**Priority: LOW** | **Effort: LOW**

Toggle controls to hide/show:
- Denied events
- Canceled events
- Past events

---

## Implementation Priority Matrix

| Recommendation | Priority | Effort | Dependencies |
|----------------|----------|--------|--------------|
| Internal Notes | HIGH | LOW | None |
| Staffing Assignments | HIGH | MEDIUM | None |
| Needs Staffing Indicators | HIGH | LOW | Staffing Assignments |
| Edit Event Capability | HIGH | MEDIUM | None |
| Payments Dashboard | HIGH | MEDIUM | None |
| Acceptance Sub-States | HIGH | MEDIUM | Schema change |
| Email Template Generation | MEDIUM | LOW | None |
| Activity Log | MEDIUM | MEDIUM | Schema change |
| Payment Ledger | MEDIUM | MEDIUM | Activity Log |
| Days Since Offer | MEDIUM | LOW | Acceptance Sub-States |
| Admin Dashboard Staffing | MEDIUM | MEDIUM | Staffing Assignments |
| Pending Actions Widget | MEDIUM | MEDIUM | Multiple |
| Meeting Mode View | MEDIUM | MEDIUM | Notes |
| Quick Comparison Calendar | MEDIUM | MEDIUM | None |
| Cancellation Flow | MEDIUM | LOW | None |
| Hold Status | LOW | LOW | None |
| Pricing Presets | LOW | LOW | Settings |
| Expected Payments View | LOW | LOW | Sub-States |
| This Week View | LOW | MEDIUM | None |
| View Filters | LOW | LOW | None |
| Export Capability | LOW | LOW | None |

---

## Recommended Implementation Sequence

### Phase 1: Essential Admin Tools
1. Internal Notes field
2. Edit Event capability  
3. Staffing Assignments + indicators

### Phase 2: Payment Visibility
4. Payments Dashboard
5. Payment Ledger/timeline

### Phase 3: Pipeline Tracking
6. Acceptance Sub-States (Offered → AwaitingDeposit → Confirmed)
7. Days Since Offer badges
8. Activity Log

### Phase 4: Polish & Efficiency
9. Email template generation
10. Pending Actions widget
11. Meeting Mode view

---

*Document created: December 2024*
*Last updated: December 2024*
