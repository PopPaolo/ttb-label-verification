import { useState } from 'react'
import '@fontsource-variable/public-sans'
import '@fontsource/ibm-plex-mono/400.css'
import '@fontsource/ibm-plex-mono/600.css'
import './App.css'
import BatchCheck from './BatchCheck'
import SingleCheck from './SingleCheck'

type Mode = 'single' | 'batch'

function App() {
  const [mode, setMode] = useState<Mode>('single')

  return (
    <div className="shell">
      <header className="masthead">
        <div>
          <p className="eyebrow">Alcohol and Tobacco Tax and Trade Bureau · Prototype</p>
          <h1>Label Verification</h1>
        </div>
        <nav className="modes" aria-label="Mode">
          <button
            type="button"
            className="mode"
            aria-pressed={mode === 'single'}
            onClick={() => setMode('single')}
          >
            Check one label
          </button>
          <button
            type="button"
            className="mode"
            aria-pressed={mode === 'batch'}
            onClick={() => setMode('batch')}
          >
            Check a batch
          </button>
        </nav>
      </header>

      {/* Both stay mounted so a running batch keeps polling while the agent
          checks a single label. */}
      <main>
        <div hidden={mode !== 'single'}>
          <SingleCheck />
        </div>
        <div hidden={mode !== 'batch'}>
          <BatchCheck />
        </div>
      </main>

      <footer className="foot">
        Results assist the examining agent — the final determination is always the agent's. Rule
        citations refer to 27 CFR.
      </footer>
    </div>
  )
}

export default App
