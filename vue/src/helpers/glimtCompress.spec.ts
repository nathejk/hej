import { describe, expect, it } from 'vitest'

import { UPLOAD_MAX_EDGE, shouldCompress, targetSize } from '@/helpers/glimtCompress'

// Upload compression decisions (task 317).
//
// The canvas work needs a DOM and is not tested; the decisions are, per the skill's rule for a `node`
// environment. These are the two that would do damage if wrong: upscaling a small photo, and
// re-encoding something that does not need it.

describe('targetSize', () => {
  it('scales a large image down to the longest edge', () => {
    expect(targetSize(4000, 3000)).toEqual({ width: UPLOAD_MAX_EDGE, height: 1536 })
    expect(targetSize(3000, 4000)).toEqual({ width: 1536, height: UPLOAD_MAX_EDGE })
  })

  // Upscaling would make this function actively harmful for exactly the uploads that are already
  // cheap: a bigger file with no more detail in it.
  it('never upscales', () => {
    expect(targetSize(800, 600)).toEqual({ width: 800, height: 600 })
    expect(targetSize(1, 1)).toEqual({ width: 1, height: 1 })
  })

  it('leaves an image exactly at the limit alone', () => {
    expect(targetSize(UPLOAD_MAX_EDGE, 1000)).toEqual({ width: UPLOAD_MAX_EDGE, height: 1000 })
  })

  it('preserves the aspect ratio', () => {
    const { width, height } = targetSize(6000, 2000)
    expect(width / height).toBeCloseTo(3, 1)
  })

  // A 4000×1 panorama scaled naively rounds its height to 0 and produces a canvas that cannot be
  // drawn — losing the photograph to an optimisation.
  it('never rounds a dimension to zero', () => {
    const { width, height } = targetSize(8000, 1)
    expect(width).toBe(UPLOAD_MAX_EDGE)
    expect(height).toBeGreaterThanOrEqual(1)
  })

  it('returns zeros for a bad decode rather than throwing', () => {
    expect(targetSize(0, 0)).toEqual({ width: 0, height: 0 })
    expect(targetSize(-1, 100)).toEqual({ width: 0, height: 0 })
  })

  it('honours a custom edge', () => {
    expect(targetSize(1000, 500, 100)).toEqual({ width: 100, height: 50 })
  })
})

describe('shouldCompress', () => {
  it('compresses a large image', () => {
    expect(shouldCompress('image/jpeg', 4 * 1024 * 1024)).toBe(true)
  })

  // A re-encoded JPEG of an already-compressed JPEG is not reliably smaller, so spending a decode
  // and an encode on a small file costs battery and may cost bytes.
  it('leaves a small image alone', () => {
    expect(shouldCompress('image/jpeg', 100 * 1024)).toBe(false)
  })

  // Re-encoding video in a canvas is not possible; task 322 owns what happens to it.
  it('never touches video', () => {
    expect(shouldCompress('video/mp4', 20 * 1024 * 1024)).toBe(false)
    expect(shouldCompress('video/quicktime', 40 * 1024 * 1024)).toBe(false)
  })

  it('ignores anything that is not an image', () => {
    expect(shouldCompress('application/pdf', 4 * 1024 * 1024)).toBe(false)
    expect(shouldCompress('', 4 * 1024 * 1024)).toBe(false)
  })
})
