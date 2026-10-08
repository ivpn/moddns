export const HOVER_LABEL_HEIGHT = 20;
/** Height of the X axis band; the hover label sits in the strip below it. */
export const AXIS_BAND_HEIGHT = 30;
export const HOVER_STRIP_HEIGHT = HOVER_LABEL_HEIGHT + 4;

const CHAR_PX = 6.4;
const PAD_PX = 16;

/** X0 and width of the time-axis hover label centred on `centerX`, kept inside [minX, maxX]. */
export function axisLabelBox(centerX: number, text: string, minX: number, maxX: number): { x: number; width: number } {
    const width = Math.round(text.length * CHAR_PX + PAD_PX);
    const x = Math.min(Math.max(centerX - width / 2, minX), Math.max(minX, maxX - width));
    return { x, width };
}
