import {describe, expect, it} from 'vitest';
import {cn} from './utils';

describe('cn', () => {
    it('merges conflicting tailwind classes, last one wins', () => {
        expect(cn('p-0', 'p-4')).toBe('p-4');
        expect(cn('text-red-500', 'text-blue-700')).toBe('text-blue-700');
    });

    it('keeps non-conflicting classes', () => {
        expect(cn('p-0', 'text-center')).toBe('p-0 text-center');
    });

    it('handles falsy and conditional values', () => {
        expect(cn('a', false && 'b', {'c': true, 'd': false})).toBe('a c');
    });
});
