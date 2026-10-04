import {act, renderHook} from '@testing-library/react';
import {describe, expect, it} from 'vitest';
import {useSidebarCollapse} from './useSidebarCollapse';

describe('useSidebarCollapse', () => {
    it('starts open', () => {
        const {result} = renderHook(() => useSidebarCollapse());
        expect(result.current.isOpen).toBe(true);
    });

    it('toggle flips the state both ways with no mounted panel', () => {
        const {result} = renderHook(() => useSidebarCollapse());

        act(() => result.current.toggle());
        expect(result.current.isOpen).toBe(false);

        act(() => result.current.toggle());
        expect(result.current.isOpen).toBe(true);
    });

    it('panel collapse and expand events sync the state', () => {
        const {result} = renderHook(() => useSidebarCollapse());

        act(() => result.current.onCollapse());
        expect(result.current.isOpen).toBe(false);

        act(() => result.current.onExpand());
        expect(result.current.isOpen).toBe(true);
    });
});
