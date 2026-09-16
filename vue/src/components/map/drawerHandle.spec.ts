import { describe, it, expect } from 'vitest'
import { drawerHandle } from './drawerHandle'

describe('drawerHandle', () => {
  // The bug the first cut had: the handle only appeared with registrations, so a patrol handed a sheet but
  // not yet scanning anything could never open the drawer to read their sticker number.
  it('appears for a patrol with handouts but no scans', () => {
    const h = drawerHandle(0, 2)
    expect(h.visible).toBe(true)
    expect(h.label).toBe('2 kort')
  })

  it('appears for a patrol with scans but no handouts', () => {
    const h = drawerHandle(3, 0)
    expect(h.visible).toBe(true)
    expect(h.label).toBe('3 registreringer')
  })

  it('shows both counts, joined', () => {
    expect(drawerHandle(3, 2).label).toBe('3 registreringer · 2 kort')
  })

  it('uses the singular for one registration', () => {
    expect(drawerHandle(1, 0).label).toBe('1 registrering')
  })

  // Both empty is the personnel / pre-race state: no handle, because opening an empty drawer is a dead end.
  it('is hidden when there is nothing to open', () => {
    const h = drawerHandle(0, 0)
    expect(h.visible).toBe(false)
    expect(h.label).toBe('')
  })
})
