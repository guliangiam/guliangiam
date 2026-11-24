import { stateCaptionKey, type EnumOption } from "@/shared/constants/enums";

/**
 * The registry's transition tables, as the console needs them.
 *
 * `web/src/shared/constants/enums.ts` answers "which values exist"; this file
 * answers "which value may follow this one". Both are the registry's facts, and
 * both are mirrored from the server's own declaration rather than retyped:
 * `scripts/check-enums.py` compares every table below against the Go
 * `var valid...Transitions map[string][]string` it is annotated with, in both
 * directions, so an edge added on the server cannot go missing here.
 *
 * Why the console needs the edges at all: a state `<Select>` that offers every
 * value in the set lets the operator pick a transition the server will refuse,
 * which is a form that cannot be submitted. The legacy console did exactly that
 * — it offered `Pending/Paid/Cancelled/Refunded` unconditionally, including the
 * `paid -> pending` edge the registry forbids.
 *
 * These tables describe the *declared* graph. A server edge declared for an
 * automated actor (a payment callback moving `payment_pending -> paid`) is not
 * an operator action, so the pages built on this module offer only what the
 * operator is allowed to do and let the server drive the rest. The tables are
 * deliberately the full graph — narrowing them here would put a policy decision
 * in the frontend.
 */
export type TransitionTable = Record<string, readonly string[]>;

/** Order transitions (`domain/orders.validOrderTransitions`, registry §1.1). */
// @lifecycle validOrderTransitions
export const ORDER_TRANSITIONS: TransitionTable = {
  created: ["payment_pending", "cancelled"],
  payment_pending: ["paid", "failed", "cancelled"],
  paid: ["completed"],
  completed: ["refunded", "closed"],
  failed: ["payment_pending"],
  cancelled: [],
  refunded: [],
  closed: [],
};

/** Payment transitions (`domain/payments.validPaymentTransitions`, registry §1.2). */
// @lifecycle validPaymentTransitions
export const PAYMENT_TRANSITIONS: TransitionTable = {
  created: ["processing", "succeeded", "failed", "cancelled"],
  processing: ["succeeded", "failed", "cancelled", "refunded", "partially_refunded"],
  succeeded: ["refunded", "partially_refunded"],
  partially_refunded: ["refunded", "partially_refunded"],
  failed: [],
  cancelled: [],
  refunded: [],
};

/** Subscription transitions (`domain/subscriptions.validSubscriptionTransitions`, §1.3). */
// @lifecycle validSubscriptionTransitions
export const SUBSCRIPTION_TRANSITIONS: TransitionTable = {
  pending: ["trialing", "active", "cancelled"],
  trialing: ["active", "past_due", "cancelled"],
  active: ["past_due", "expired", "cancelled"],
  past_due: ["active", "expired", "cancelled"],
  expired: [],
  cancelled: [],
};

/** Transaction transitions (`domain/transactions.validTransactionTransitions`, §1.4). */
// @lifecycle validTransactionTransitions
export const TRANSACTION_TRANSITIONS: TransitionTable = {
  pending: ["completed", "failed", "cancelled"],
  completed: [],
  failed: [],
  cancelled: [],
};

/** Coupon transitions (`domain/coupons.validCouponTransitions`, registry §1.5). */
// @lifecycle validCouponTransitions
export const COUPON_TRANSITIONS: TransitionTable = {
  active: ["disabled", "expired"],
  disabled: ["active", "expired"],
  expired: [],
};

/** Product transitions (`domain/products.validProductTransitions`, registry §1.5). */
// @lifecycle validProductTransitions
export const PRODUCT_TRANSITIONS: TransitionTable = {
  draft: ["active", "discontinued"],
  active: ["discontinued"],
  discontinued: [],
};

/** Plan transitions (`domain/plans.validPlanTransitions`, registry §1.5). */
// @lifecycle validPlanTransitions
export const PLAN_TRANSITIONS: TransitionTable = {
  draft: ["active", "archived"],
  active: ["archived"],
  archived: [],
};

/** Ticket transitions (`domain/tickets.validTicketTransitions`, registry §1.5). */
// @lifecycle validTicketTransitions
export const TICKET_TRANSITIONS: TransitionTable = {
  open: ["pending", "resolved", "closed"],
  pending: ["open", "resolved", "closed"],
  resolved: ["pending", "closed"],
  closed: [],
};

/**
 * Options for a state select: the current value plus every value the registry
 * lets it move to, in the order the registry set declares them.
 *
 * The current value is included so the select shows what the record *is* rather
 * than going blank; resubmitting it is not a transition, and the server treats an
 * unchanged value as a no-op. A value with no outgoing edge (a terminal state)
 * yields a single option, and the caller disables the select — a terminal state
 * is by definition irreversible.
 */
export function allowedTargets(
  values: readonly string[],
  from: string,
  transitions: TransitionTable,
  translate: (key: string) => string,
): EnumOption[] {
  const allowed = new Set([from, ...(transitions[from] ?? [])]);
  return values
    .filter((value) => allowed.has(value))
    .map((value) => ({ label: translate(stateCaptionKey(value)), value }));
}

/** Whether the registry declares no outgoing edge, i.e. the state is terminal. */
export function isTerminal(from: string, transitions: TransitionTable): boolean {
  return (transitions[from] ?? []).length === 0;
}
