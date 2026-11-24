import {
  iamList,
  iamGet,
  iamCreate,
  iamPatch,
  iamDelete,
  iamPost,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import { TICKETS, TICKET_BY_ID, TICKET_MESSAGES } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Tickets
// ─────────────────────────────────────────────
export type IamTicketItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  type: string;
  status: string;
  priority: string;
  assignee: string;
  creator: string;
  createdAt: string;
  updatedAt: string;
};
export type IamTicketDetail = IamTicketItem & {
  assignee: string;
  creator: string;
  description: string;
  attachment: string;
  resolvedAt: string | null;
  revision: number;
};
export type IamCreateTicketPayload = {
  /** Optional: honoured only for platform admins; members are scoped server-side. */
  organizationId?: string;
  name: string;
  displayName: string;
  type?: string;
  status?: string;
  priority?: string;
  assignee?: string;
  creator?: string;
  description?: string;
  attachment?: string;
};
export type IamPatchTicketPayload = {
  displayName?: string;
  type?: string;
  status?: string;
  priority?: string;
  assignee?: string;
  description?: string;
  attachment?: string;
  revision: number;
};
export type IamTicketMessagePayload = {
  author: string;
  content: string;
  attachment?: string;
};

/**
 * Append the trusted-tenant override to a ticket endpoint.
 *
 * `organizationId` is honoured by the server only for platform administrators
 * (cross-organization access); for a member it must match the session scope,
 * so callers pass the row's own organization and the server re-adjudicates it.
 */
function withOrgQuery(path: string, organizationId?: string): string {
  if (!organizationId) return path;
  const separator = path.includes("?") ? "&" : "?";
  return `${path}${separator}organizationId=${encodeURIComponent(organizationId)}`;
}

export const getIamTickets = (params?: IamListParams) => iamList<IamTicketItem>(TICKETS, params);
export const getIamTicketsPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamTicketItem>(TICKETS, params);
export const getIamTicket = (id: string, organizationId?: string) =>
  iamGet<IamTicketDetail>(withOrgQuery(TICKET_BY_ID(id), organizationId));
export const createIamTicket = (p: IamCreateTicketPayload) => iamCreate(TICKETS, p);
export const patchIamTicket = (id: string, p: IamPatchTicketPayload, organizationId?: string) =>
  iamPatch(withOrgQuery(TICKET_BY_ID(id), organizationId), p);
export const deleteIamTicket = (id: string, organizationId?: string) =>
  iamDelete(withOrgQuery(TICKET_BY_ID(id), organizationId));

/** Add a message to a ticket, returning the updated ticket detail. */
export const addTicketMessage = (
  ticketId: string,
  payload: IamTicketMessagePayload,
  organizationId?: string,
) => iamPost<IamTicketDetail>(withOrgQuery(TICKET_MESSAGES(ticketId), organizationId), payload);

/** Ticket message type. */
export type IamTicketMessage = {
  author: string;
  content: string;
  attachment?: string;
  createdAt: string;
};

/** List messages for a ticket. */
export const listTicketMessages = (ticketId: string, params?: IamListParams) =>
  iamList<IamTicketMessage>(`${TICKET_MESSAGES(ticketId)}`, params);
