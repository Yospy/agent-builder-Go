# Delete API Contract

## Delete Agent

`DELETE /api/agents/{id}`

Deletes a non-builder agent from live state. All chats for that agent are deleted in the same operation.

Responses:
- `204 No Content` — deleted
- `400 {"error":"agent builder cannot be deleted"}` — `{id}` is `builder`
- `404 {"error":"agent not found"}` — no agent exists for `{id}`
- `409 {"error":"a run is already in flight for this agent"}` — one of the agent's chats is currently running
- `500 {"error":"could not delete agent"}` — unexpected server/store failure

Frontend rules:
- Do not show delete UI for `agent.id === "builder"`.
- Disable or hide delete while any known chat for that agent is actively streaming/running when possible.
- If `409`, keep the agent visible and show a retry-after-run message.
- After `204`, remove the agent from local agent lists and clear/navigate away from any chat for that agent.
- Treat `404` as already gone and refresh local state.

## Delete Chat

`DELETE /api/sessions/{id}`

Deletes one chat/session from live state.

Responses:
- `204 No Content` — deleted
- `404 {"error":"session not found"}` — no chat exists for `{id}`
- `409 {"error":"a run is already in flight for this session"}` — chat is currently running
- `500 {"error":"could not delete session"}` — unexpected server/store failure

Frontend rules:
- Disable or hide delete while the chat is actively streaming/running when possible.
- If `409`, keep the chat visible and show a retry-after-run message.
- After `204`, remove the chat from local recent/history lists and navigate to another valid chat or an empty state.
- Treat `404` as already gone and refresh local state.

## Notes

- Deletion is hard-delete from SQLite live state.
- Append-only audit logs under `logs/` are not deleted.
- Successful `204` responses have no JSON body.
