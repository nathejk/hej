// Client-side image compression before upload (PRD 019 §8, task 317).
//
// # Why the client compresses at all, when the server already does
//
// The BFF re-encodes everything it stores (task 303), so this is not about what ends up on disk. It
// is about what travels: a modern phone photograph is 3–8 MB, a glimt carries up to ten of them, and
// the upload happens from a field at night on one bar of signal. Sending originals would mean a
// member watching a progress bar for several minutes and a post that fails halfway more often than
// it succeeds.
//
// The server still enforces its own limits and does its own re-encode, because a client-side
// reduction is a courtesy and never a control.
//
// # The decision is separated from the canvas work
//
// `targetSize()` is pure and tested; `compressImage()` needs a DOM and is not. That split is the
// skill's rule for a `node` test environment: extract the decision, leave the plumbing.

/** The longest edge we upload. Above the server's 1600px display size, so the server still decides. */
export const UPLOAD_MAX_EDGE = 2048

/** JPEG quality for the re-encode. */
export const UPLOAD_QUALITY = 0.82

/**
 * The dimensions to scale an image to.
 *
 * **Never upscales.** A small photograph stays small rather than being blown up into a larger file
 * with no more detail in it — which would make this function actively harmful for exactly the
 * uploads that are already cheap.
 *
 * Preserves the aspect ratio, and rounds so no dimension lands on zero: a 4000×1 panorama scaled
 * naively would round its height to 0 and produce a canvas that cannot be drawn.
 */
export function targetSize(
  width: number,
  height: number,
  maxEdge: number = UPLOAD_MAX_EDGE,
): { width: number; height: number } {
  if (width <= 0 || height <= 0) return { width: 0, height: 0 }
  const longest = Math.max(width, height)
  if (longest <= maxEdge) return { width, height }

  const scale = maxEdge / longest
  return {
    width: Math.max(1, Math.round(width * scale)),
    height: Math.max(1, Math.round(height * scale)),
  }
}

/**
 * Whether a file is worth compressing.
 *
 * Videos are passed through untouched: re-encoding one in a canvas is not possible, and task 322
 * owns what happens to them. A file already under the threshold is passed through too — spending a
 * decode and a re-encode to save nothing costs a phone battery and can make the file *bigger*, since
 * a re-encoded JPEG of an already-compressed JPEG is not reliably smaller.
 */
export function shouldCompress(type: string, bytes: number, threshold = 512 * 1024): boolean {
  return type.startsWith('image/') && bytes > threshold
}

/**
 * What compression produced: the bytes to upload, and the pixel dimensions of them.
 *
 * The dimensions are returned rather than discarded because the **outbox needs them** (task 325). A
 * queued glimt renders from its local Blob before the server has ever seen it, and without
 * dimensions the card falls back to a 4:3 box — which crops a portrait photograph badly and then
 * *changes shape* once the upload returns the real values. Seen on a device, 2026-09-17.
 *
 * `width`/`height` are **0 when unknown**, which is a real case: a file small enough to skip
 * compression is never decoded, and decoding one purely to measure it would spend a decode on every
 * small image to improve one card's first paint. The card treats 0 the way it already treats a media
 * row with no dimensions.
 */
export interface CompressedImage {
  blob: Blob
  /** 0 when not known — see above. */
  width: number
  /** 0 when not known. */
  height: number
}

/**
 * Downscale and re-encode an image file.
 *
 * Returns the original file when anything at all goes wrong — an unsupported format, a decode
 * failure, a canvas the browser refuses to allocate for a very large image. The server can handle
 * the original, so falling back costs bandwidth rather than the post. Failing here would lose a
 * photograph over an optimisation.
 *
 * Uses `createImageBitmap` where available: it decodes off the main thread, which on a mid-range
 * Android phone is the difference between a composer that stutters while you add ten photos and one
 * that does not.
 */
export async function compressImage(file: File): Promise<CompressedImage> {
  const original = (): CompressedImage => ({ blob: file, width: 0, height: 0 })

  if (!shouldCompress(file.type, file.size)) return original()
  if (typeof document === 'undefined') return original()

  try {
    const bitmap = await decode(file)
    // The source dimensions, kept before the bitmap is closed. Reported even when the re-encode is
    // discarded below: the *pixels* are the same either way, and a card that knows the shape of the
    // original is exactly as correct as one that knows the shape of the copy.
    const source = { width: bitmap.width, height: bitmap.height }
    const { width, height } = targetSize(bitmap.width, bitmap.height)
    if (width === 0 || height === 0) return original()

    const canvas = document.createElement('canvas')
    canvas.width = width
    canvas.height = height
    const ctx = canvas.getContext('2d')
    if (!ctx) return { blob: file, ...source }
    ctx.drawImage(bitmap, 0, 0, width, height)
    if ('close' in bitmap) bitmap.close()

    const blob = await new Promise<Blob | null>((resolve) =>
      canvas.toBlob(resolve, 'image/jpeg', UPLOAD_QUALITY),
    )
    // Only take the re-encode if it actually helped. A re-encoded JPEG is not reliably smaller than
    // its source, and uploading a *larger* file than the member chose would be the opposite of the
    // point.
    if (!blob || blob.size >= file.size) return { blob: file, ...source }
    return { blob, width, height }
  } catch {
    return original()
  }
}

/**
 * Read an image's pixel dimensions, or `{0, 0}` if it cannot be read.
 *
 * Exists for the outbox (task 325): a queued glimt is rendered from its local Blob before the server
 * has measured anything, and a card with no dimensions falls back to a 4:3 box — which crops a
 * portrait photograph and then reshapes when the upload response arrives. One decode at the moment
 * the member taps *Del* is invisible next to the upload that follows.
 *
 * Never throws. Dimensions are a presentation nicety; a post must not fail for want of them.
 */
export async function measureImage(blob: Blob): Promise<{ width: number; height: number }> {
  const unknown = { width: 0, height: 0 }
  if (!blob.type.startsWith('image/')) return unknown

  try {
    if (typeof createImageBitmap === 'function') {
      const bitmap = await createImageBitmap(blob)
      const size = { width: bitmap.width, height: bitmap.height }
      bitmap.close()
      return size
    }
    if (typeof document === 'undefined') return unknown
    const url = URL.createObjectURL(blob)
    try {
      const img = new Image()
      await new Promise<void>((resolve, reject) => {
        img.onload = () => resolve()
        img.onerror = () => reject(new Error('decode failed'))
        img.src = url
      })
      return { width: img.naturalWidth, height: img.naturalHeight }
    } finally {
      URL.revokeObjectURL(url)
    }
  } catch {
    return unknown
  }
}

async function decode(file: File): Promise<ImageBitmap | HTMLImageElement> {
  if (typeof createImageBitmap === 'function') {
    return await createImageBitmap(file)
  }
  const url = URL.createObjectURL(file)
  try {
    return await new Promise<HTMLImageElement>((resolve, reject) => {
      const img = new Image()
      img.onload = () => resolve(img)
      img.onerror = () => reject(new Error('decode failed'))
      img.src = url
    })
  } finally {
    URL.revokeObjectURL(url)
  }
}
