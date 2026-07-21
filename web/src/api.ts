import type { Application, BatchItem, BatchSummary, VerifyResponse } from './types'

// All endpoints return { error: string } on failure; surface that message
// verbatim — the backend already phrases it for the agent.
async function parse<T>(res: Response): Promise<T> {
  let body: unknown
  try {
    body = await res.json()
  } catch {
    throw new Error('The server sent an unexpected response. Try again in a moment.')
  }
  if (!res.ok) {
    const msg = (body as { error?: string }).error
    throw new Error(msg || `Request failed (${res.status})`)
  }
  return body as T
}

export async function verifyLabel(app: Application, images: File[]): Promise<VerifyResponse> {
  const form = new FormData()
  form.append('application', JSON.stringify(app))
  for (const img of images) form.append('images', img)
  return parse(await fetch('/api/verify', { method: 'POST', body: form }))
}

export async function submitBatch(zip: File): Promise<{ id: string }> {
  const form = new FormData()
  form.append('batch', zip)
  return parse(await fetch('/api/batches', { method: 'POST', body: form }))
}

export async function getBatch(id: string): Promise<BatchSummary> {
  return parse(await fetch(`/api/batches/${encodeURIComponent(id)}`))
}

export async function getBatchItem(batchID: string, itemID: string): Promise<BatchItem> {
  return parse(
    await fetch(`/api/batches/${encodeURIComponent(batchID)}/items/${encodeURIComponent(itemID)}`),
  )
}
