import {vi} from 'vitest';
import '@testing-library/jest-dom/vitest';

// jsdom has no matchMedia; sonner/next-themes call it on mount.
Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn()
    }))
});

// jsdom measures every element as 0x0, which leaves react-virtual
// range-less (it reads offsetWidth/offsetHeight and never resizes). The
// scroll container of a virtualized list opts into a stub viewport through
// data-virtual-scroll; everything else keeps jsdom zeros, so row
// measurement passes fall back to estimateSize as after first layout.
Object.defineProperty(HTMLElement.prototype, 'offsetHeight', {
    configurable: true,
    get(this: HTMLElement) {
        if (this.hasAttribute('data-virtual-scroll')) {
            return 768;
        }
        return this.hasAttribute('data-index') ? 36 : 0;
    }
});

Object.defineProperty(HTMLElement.prototype, 'offsetWidth', {
    configurable: true,
    get(this: HTMLElement) {
        if (this.hasAttribute('data-virtual-scroll') || this.hasAttribute('data-index')) {
            return 1024;
        }
        return 0;
    }
});

// jsdom has no ResizeObserver; react-virtual would otherwise keep estimate
// sizes only. This stub reports the same sizes the offset getters expose,
// synchronously, so measurement never contradicts the estimate.
globalThis.ResizeObserver = class {
    private readonly cb: (entries: ResizeObserverEntry[], obs: ResizeObserver) => void;

    constructor(cb: (entries: ResizeObserverEntry[], obs: ResizeObserver) => void) {
        this.cb = cb;
    }

    observe(el: Element) {
        const height = el instanceof HTMLElement ? el.offsetHeight : 0;
        const width = el instanceof HTMLElement ? el.offsetWidth : 0;
        this.cb([{
            target: el,
            borderBoxSize: [{inlineSize: width, blockSize: height}],
            contentRect: {width, height} as DOMRectReadOnly,
            contentBoxSize: null,
            devicePixelContentBoxSize: null
        } as unknown as ResizeObserverEntry], this);
    }

    unobserve() {
    }

    disconnect() {
    }
};

// jsdom lacks the pointer-capture APIs; Radix menus call them on pointerdown.
Element.prototype.hasPointerCapture = () => false;
Element.prototype.setPointerCapture = () => {
};
Element.prototype.releasePointerCapture = () => {
};

// The generated Wails runtime bridge expects window.runtime, and its
// EventsOn delegates to EventsOnMultiple; this stand-in keeps jsdom tests
// rendering. configurable lets event-specific tests replace it wholesale.
Object.defineProperty(window, 'runtime', {
    writable: true,
    configurable: true,
    value: {
        EventsOn: () => () => {
        },
        EventsOnMultiple: () => () => {
        },
        EventsOff: () => {
        }
    }
});
