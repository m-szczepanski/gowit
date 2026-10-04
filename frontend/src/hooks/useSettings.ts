import {useEffect} from 'react';
import {GetSettings} from '../../wailsjs/go/main/App';

// Loads persisted settings at app start and applies the theme to <html>.
// Until issue #14 adds a system theme, anything that is not "dark" renders
// light.
export function useSettings() {
    useEffect(() => {
        GetSettings()
            .then((loaded) => {
                document.documentElement.classList.toggle('dark', loaded.theme === 'dark');
            })
            .catch(() => {
                // defaults match the shipped dark palette; issue #14 surfaces
                // load failures in the settings screen
            });
    }, []);
}
