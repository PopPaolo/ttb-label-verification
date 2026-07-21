import { useCallback, useEffect, useRef, useState } from 'react'
import { getBatch, getBatchItem, submitBatch } from './api'
import ReportView from './ReportView'
import type { BatchItem, BatchItemSummary, BatchSummary } from './types'

const POLL_MS = 1500

// Problem labels first: that is the whole point of batch triage (SPEC §6).
const severity: Record<string, number> = {
  fail: 0,
  failed: 1,
  needs_review: 2,
  processing: 3,
  queued: 4,
  pass: 5,
}

function rank(item: BatchItemSummary): number {
  const key = item.status === 'done' ? item.report_status ?? 'pass' : item.status
  return severity[key] ?? 3
}

function itemChip(item: BatchItemSummary): { cls: string; label: string } {
  if (item.status === 'done') {
    const s = item.report_status ?? 'pass'
    return {
      cls: s,
      label: s === 'pass' ? 'Pass' : s === 'fail' ? 'Fail' : 'Needs review',
    }
  }
  if (item.status === 'failed') return { cls: 'failed', label: 'Error' }
  return { cls: item.status, label: item.status === 'queued' ? 'Queued' : 'Processing' }
}

export default function BatchCheck() {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [batch, setBatch] = useState<BatchSummary | null>(null)
  const [selected, setSelected] = useState<BatchItem | null>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const poll = useCallback(async (id: string) => {
    try {
      const summary = await getBatch(id)
      setBatch(summary)
      if (summary.pending > 0) {
        timer.current = setTimeout(() => poll(id), POLL_MS)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Lost track of the batch. Submit it again.')
    }
  }, [])

  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current)
    },
    [],
  )

  async function submit(file: File) {
    setError('')
    setBusy(true)
    try {
      const { id } = await submitBatch(file)
      await poll(id)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong. Try again.')
    } finally {
      setBusy(false)
    }
  }

  async function open(itemID: string) {
    if (!batch) return
    setError('')
    try {
      setSelected(await getBatchItem(batch.id, itemID))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not load that label. Try again.')
    }
  }

  function reset() {
    if (timer.current) clearTimeout(timer.current)
    setBatch(null)
    setSelected(null)
    setError('')
  }

  // Drill-in view for one label of the batch.
  if (selected) {
    return (
      <div className="card">
        <div className="actions" style={{ marginTop: 0, marginBottom: '1.25rem' }}>
          <button type="button" className="btn-ghost" onClick={() => setSelected(null)}>
            ← Back to batch
          </button>
          <span className="report-meta">application {selected.id}</span>
        </div>
        {selected.report ? (
          <ReportView
            status={selected.report.status}
            results={selected.report.results}
            extraction={selected.extraction ?? null}
          />
        ) : (
          <p className="notice-error" role="alert">
            {selected.error || 'This label has not finished processing yet.'}
          </p>
        )}
      </div>
    )
  }

  if (!batch) {
    return (
      <div className="card">
        <h2>Check a batch of applications</h2>
        <div className="uploader">
          <label className="add-images">
            <span>{busy ? 'Uploading…' : 'Choose a batch zip file'}</span>
            <span className="sub">One zip with the label images and a manifest.</span>
            <input
              type="file"
              accept=".zip,application/zip"
              hidden
              disabled={busy}
              onChange={(e) => {
                const f = e.target.files?.[0]
                if (f) submit(f)
                e.target.value = ''
              }}
            />
          </label>
        </div>
        <p className="zip-help">
          The zip needs a <code>manifest.json</code> listing each application's fields and its image
          files: <code>{'{"applications": [{"id": "app-1", "images": ["app-1-front.png"], …}]}'}</code>{' '}
          — each entry takes the same fields as the single-label form.
        </p>
        {error && (
          <p className="notice-error" role="alert">
            {error}
          </p>
        )}
      </div>
    )
  }

  const items = [...batch.items].sort((a, b) => rank(a) - rank(b) || a.id.localeCompare(b.id))
  const done = batch.items.filter((i) => i.status === 'done')
  const nFail = done.filter((i) => i.report_status === 'fail').length
  const nReview = done.filter((i) => i.report_status === 'needs_review').length
  const nPass = done.filter((i) => i.report_status === 'pass').length
  const nError = batch.items.filter((i) => i.status === 'failed').length

  return (
    <div className="card">
      <div className="report-head">
        <h2>
          Batch of {batch.total} label{batch.total === 1 ? '' : 's'}
        </h2>
        {batch.pending > 0 ? (
          <span className="busy" role="status">
            <span className="spinner" aria-hidden="true" />
            {batch.pending} still processing
          </span>
        ) : (
          <button type="button" className="btn-ghost" onClick={reset}>
            Start another batch
          </button>
        )}
      </div>

      <div className="counts">
        <div className="count fail">
          <strong>{nFail}</strong>
          <span>Fail</span>
        </div>
        <div className="count needs_review">
          <strong>{nReview}</strong>
          <span>Needs review</span>
        </div>
        <div className="count pass">
          <strong>{nPass}</strong>
          <span>Pass</span>
        </div>
        {nError > 0 && (
          <div className="count fail">
            <strong>{nError}</strong>
            <span>Could not process</span>
          </div>
        )}
      </div>

      <div className="batch-list">
        {items.map((item) => {
          const chip = itemChip(item)
          const clickable = item.status === 'done' || item.status === 'failed'
          return (
            <button
              key={item.id}
              type="button"
              className="batch-item"
              onClick={() => open(item.id)}
              disabled={!clickable}
            >
              <span className="item-id">{item.id}</span>
              {item.error && <span className="item-err">{item.error}</span>}
              <span className={`chip ${chip.cls}`}>{chip.label}</span>
              <span className="chev" aria-hidden="true">
                {clickable ? '›' : ''}
              </span>
            </button>
          )
        })}
      </div>

      {error && (
        <p className="notice-error" role="alert">
          {error}
        </p>
      )}
    </div>
  )
}
