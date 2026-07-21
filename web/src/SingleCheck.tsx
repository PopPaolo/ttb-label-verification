import { useEffect, useRef, useState } from 'react'
import { verifyLabel } from './api'
import ReportView from './ReportView'
import type { Application, BeverageType, VerifyResponse } from './types'

const MAX_IMAGES = 6

interface Picked {
  file: File
  url: string
}

export default function SingleCheck() {
  const [beverage, setBeverage] = useState<BeverageType>('wine')
  const [brand, setBrand] = useState('')
  const [fanciful, setFanciful] = useState('')
  const [classType, setClassType] = useState('')
  const [abv, setAbv] = useState('')
  const [netContents, setNetContents] = useState('')
  const [nameAddress, setNameAddress] = useState('')
  const [imported, setImported] = useState(false)
  const [appellation, setAppellation] = useState('')
  const [varietals, setVarietals] = useState('')
  const [vintage, setVintage] = useState('')
  const [formula, setFormula] = useState('')

  const [images, setImages] = useState<Picked[]>([])
  const [dragging, setDragging] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [result, setResult] = useState<VerifyResponse | null>(null)
  const reportRef = useRef<HTMLDivElement>(null)

  useEffect(() => () => images.forEach((i) => URL.revokeObjectURL(i.url)), [images])

  function addFiles(files: FileList | File[]) {
    const picked = [...files]
      .filter((f) => f.type.startsWith('image/'))
      .map((file) => ({ file, url: URL.createObjectURL(file) }))
    setImages((cur) => [...cur, ...picked].slice(0, MAX_IMAGES))
  }

  function removeImage(idx: number) {
    setImages((cur) => {
      URL.revokeObjectURL(cur[idx].url)
      return cur.filter((_, i) => i !== idx)
    })
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setError('')

    if (images.length === 0) {
      setError('Add at least one label image before verifying.')
      return
    }
    const abvNum = Number(abv)
    if (abv === '' || Number.isNaN(abvNum)) {
      setError('Enter the alcohol content from the application as a number, like 13.5.')
      return
    }

    const app: Application = {
      beverage_type: beverage,
      brand_name: brand.trim(),
      fanciful_name: fanciful.trim() || undefined,
      class_type: classType.trim(),
      alcohol_content: abvNum,
      net_contents: netContents.trim(),
      name_address: nameAddress.trim(),
      imported,
      formula_number: formula.trim() || undefined,
    }
    if (beverage === 'wine') {
      const list = varietals
        .split(',')
        .map((v) => v.trim())
        .filter(Boolean)
      app.appellation = appellation.trim() || undefined
      app.varietals = list.length > 0 ? list : undefined
      app.vintage_date = vintage.trim() || undefined
    }

    setBusy(true)
    setResult(null)
    try {
      const res = await verifyLabel(app, images.map((i) => i.file))
      setResult(res)
      requestAnimationFrame(() =>
        reportRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' }),
      )
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong. Try again.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <form onSubmit={submit}>
        <section className="card">
          <h2>Application data</h2>
          <div className="form-grid">
            <div className="field">
              <label htmlFor="beverage">Beverage type</label>
              <select
                id="beverage"
                value={beverage}
                onChange={(e) => setBeverage(e.target.value as BeverageType)}
              >
                <option value="wine">Wine</option>
                <option value="malt">Malt beverage</option>
                <option value="distilled_spirits">Distilled spirits</option>
              </select>
            </div>
            <div className="field">
              <label htmlFor="brand">Brand name</label>
              <input
                id="brand"
                value={brand}
                onChange={(e) => setBrand(e.target.value)}
                required
                placeholder="Stone's Throw"
              />
            </div>
            <div className="field">
              <label htmlFor="fanciful">
                Fanciful name <span className="why">(if any)</span>
              </label>
              <input id="fanciful" value={fanciful} onChange={(e) => setFanciful(e.target.value)} />
            </div>
            <div className="field">
              <label htmlFor="classType">Class / type designation</label>
              <input
                id="classType"
                value={classType}
                onChange={(e) => setClassType(e.target.value)}
                placeholder="Cabernet Sauvignon"
              />
            </div>
            <div className="field">
              <label htmlFor="abv">Alcohol content (% ABV)</label>
              <input
                id="abv"
                value={abv}
                onChange={(e) => setAbv(e.target.value)}
                inputMode="decimal"
                placeholder="13.5"
              />
            </div>
            <div className="field">
              <label htmlFor="net">Net contents</label>
              <input
                id="net"
                value={netContents}
                onChange={(e) => setNetContents(e.target.value)}
                placeholder="750 mL"
              />
            </div>
            <div className="field wide">
              <label htmlFor="addr">Name and address of bottler / producer / importer</label>
              <input
                id="addr"
                value={nameAddress}
                onChange={(e) => setNameAddress(e.target.value)}
                placeholder="Bottled by Example Cellars, Napa, CA"
              />
            </div>
            <div className="field">
              <label htmlFor="formula">
                Formula number <span className="why">(if a formula is required)</span>
              </label>
              <input id="formula" value={formula} onChange={(e) => setFormula(e.target.value)} />
            </div>
            <div className="field">
              <label className="checkline" htmlFor="imported">
                <input
                  id="imported"
                  type="checkbox"
                  checked={imported}
                  onChange={(e) => setImported(e.target.checked)}
                />
                Imported product
              </label>
            </div>

            {beverage === 'wine' && (
              <>
                <span className="subhead">Wine details (where applicable)</span>
                <div className="field">
                  <label htmlFor="appellation">Appellation of origin</label>
                  <input
                    id="appellation"
                    value={appellation}
                    onChange={(e) => setAppellation(e.target.value)}
                    placeholder="Napa Valley"
                  />
                </div>
                <div className="field">
                  <label htmlFor="varietals">
                    Grape varietal(s) <span className="why">(comma-separated)</span>
                  </label>
                  <input
                    id="varietals"
                    value={varietals}
                    onChange={(e) => setVarietals(e.target.value)}
                    placeholder="Cabernet Sauvignon, Merlot"
                  />
                </div>
                <div className="field">
                  <label htmlFor="vintage">Vintage date</label>
                  <input
                    id="vintage"
                    value={vintage}
                    onChange={(e) => setVintage(e.target.value)}
                    placeholder="2023"
                  />
                </div>
              </>
            )}
          </div>
        </section>

        <section className="card">
          <h2>Label images</h2>
          <div className="uploader">
            <label
              className={`add-images${dragging ? ' dragging' : ''}`}
              onDragOver={(e) => {
                e.preventDefault()
                setDragging(true)
              }}
              onDragLeave={() => setDragging(false)}
              onDrop={(e) => {
                e.preventDefault()
                setDragging(false)
                addFiles(e.dataTransfer.files)
              }}
            >
              <span>Add label images</span>
              <span className="sub">
                Click to choose files or drag them here.
                <br />
                Up to {MAX_IMAGES} pieces — front, back, neck, strip.
              </span>
              <input
                type="file"
                accept="image/*"
                multiple
                hidden
                onChange={(e) => {
                  if (e.target.files) addFiles(e.target.files)
                  e.target.value = ''
                }}
              />
            </label>
            {images.map((img, i) => (
              <figure key={img.url} className="thumb">
                <img src={img.url} alt={`Label piece ${i + 1}: ${img.file.name}`} />
                <figcaption>
                  {i + 1} · {img.file.name}
                </figcaption>
                <button
                  type="button"
                  className="remove"
                  onClick={() => removeImage(i)}
                  aria-label={`Remove ${img.file.name}`}
                >
                  ×
                </button>
              </figure>
            ))}
          </div>

          <div className="actions">
            <button type="submit" className="btn-primary" disabled={busy}>
              Verify label
            </button>
            {busy && (
              <span className="busy" role="status">
                <span className="spinner" aria-hidden="true" />
                Reading the label — usually under 5 seconds
              </span>
            )}
          </div>
          {error && (
            <p className="notice-error" role="alert">
              {error}
            </p>
          )}
        </section>
      </form>

      {result && (
        <div className="card" ref={reportRef}>
          <ReportView
            status={result.status}
            results={result.results}
            extraction={result.extraction}
            timingsMS={result.timings_ms}
            imageNames={images.map((i) => i.file.name)}
          />
        </div>
      )}
    </>
  )
}
