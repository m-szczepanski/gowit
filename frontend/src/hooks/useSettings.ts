import {useEffect, useState} from 'react';
import {GetSettings} from '../../wailsjs/go/main/App';
import type {config} from '../../wailsjs/go/models';

export type Settings = config.Settings;

// Issue #12: settings load at startup and the theme applies to <html>.
// Until issue #14 adds a system theme, anything that is not "dark" renders
// light. The settings screen will consume the returned value and the
// save path.
function applyTheme(theme: string) {
    document.documentElement.classList.toggle('dark', theme === 'dark');
}

export function useSettings() {
    const [settings, setSettings] = useState<Settings | null>(null);

    useEffect(() => {
        GetSettings()
            .then((loaded) => {
                applyTheme(loaded.theme);
                setSettings(loaded);
            })
            .catch(() => {
                // defaults already match the shipped dark palette; issue #14
                // surfaces load failures once the settings screen exists
            });
    }, []);

    return settings;
}
