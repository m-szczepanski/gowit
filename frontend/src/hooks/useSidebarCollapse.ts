import {useCallback, useRef, useState} from 'react';
import type {ImperativePanelHandle} from 'react-resizable-panels';

export const SIDEBAR_PANEL_ID = 'sidebar-panel';

/**
 * Single source of truth for sidebar open/closed state. Button toggles call
 * the panel ref; panel-driven changes (drag collapse/expand) arrive through
 * onCollapse/onExpand so aria-expanded never desyncs from the layout.
 */
export function useSidebarCollapse() {
    const panelRef = useRef<ImperativePanelHandle>(null);
    const [isOpen, setIsOpen] = useState(true);

    const onCollapse = useCallback(() => setIsOpen(false), []);
    const onExpand = useCallback(() => setIsOpen(true), []);

    const toggle = useCallback(() => {
        const next = !isOpen;
        setIsOpen(next);
        if (next) {
            panelRef.current?.expand();
        } else {
            panelRef.current?.collapse();
        }
    }, [isOpen]);

    return {panelRef, isOpen, toggle, onCollapse, onExpand};
}
