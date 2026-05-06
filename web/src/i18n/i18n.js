/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';

import { normalizeLanguage, supportedLanguages } from './language';

const DEFAULT_LANGUAGE = 'zh-CN';
const DEFAULT_NAMESPACE = 'translation';

const localeLoaders = {
  en: () => import('./locales/en.json'),
  fr: () => import('./locales/fr.json'),
  'zh-CN': () => import('./locales/zh-CN.json'),
  'zh-TW': () => import('./locales/zh-TW.json'),
  ru: () => import('./locales/ru.json'),
  ja: () => import('./locales/ja.json'),
  vi: () => import('./locales/vi.json'),
};

const loadedLanguages = new Set();
let initializePromise;
let changeLanguagePatched = false;

const isSupportedLanguage = (language) => supportedLanguages.includes(language);

const resolveSupportedLanguage = (language) => {
  const normalized = normalizeLanguage(language);
  return isSupportedLanguage(normalized) ? normalized : DEFAULT_LANGUAGE;
};

const getStoredLanguage = () => {
  if (typeof window === 'undefined') {
    return undefined;
  }

  try {
    return window.localStorage?.getItem('i18nextLng');
  } catch (error) {
    console.error('[i18n] failed to read stored language', error);
    return undefined;
  }
};

const getBrowserLanguage = () => {
  if (typeof navigator === 'undefined') {
    return undefined;
  }
  return navigator.languages?.[0] || navigator.language;
};

export const getInitialLanguage = () =>
  resolveSupportedLanguage(getStoredLanguage() || getBrowserLanguage());

const unwrapLocaleModule = (module) => module.default || module;

const importLocaleResource = async (language) => {
  const resolvedLanguage = resolveSupportedLanguage(language);
  const localeModule = await localeLoaders[resolvedLanguage]();
  const resource = unwrapLocaleModule(localeModule);

  return {
    language: resolvedLanguage,
    resource:
      resource[DEFAULT_NAMESPACE] !== undefined
        ? resource[DEFAULT_NAMESPACE]
        : resource,
  };
};

export const loadLanguageResource = async (language) => {
  const resolvedLanguage = resolveSupportedLanguage(language);
  if (loadedLanguages.has(resolvedLanguage)) {
    return resolvedLanguage;
  }

  const { resource } = await importLocaleResource(resolvedLanguage);
  i18n.addResourceBundle(
    resolvedLanguage,
    DEFAULT_NAMESPACE,
    resource,
    true,
    true,
  );
  loadedLanguages.add(resolvedLanguage);

  return resolvedLanguage;
};

const loadLanguageResourceWithFallback = async (language) => {
  try {
    return await loadLanguageResource(language);
  } catch (error) {
    const resolvedLanguage = resolveSupportedLanguage(language);
    console.error(`[i18n] failed to load ${resolvedLanguage} locale`, error);

    if (resolvedLanguage === DEFAULT_LANGUAGE) {
      throw error;
    }

    return loadLanguageResource(DEFAULT_LANGUAGE);
  }
};

const patchChangeLanguage = () => {
  if (changeLanguagePatched) {
    return;
  }

  const originalChangeLanguage = i18n.changeLanguage.bind(i18n);
  i18n.changeLanguage = async (language, callback) => {
    const resolvedLanguage = await loadLanguageResourceWithFallback(language);
    return originalChangeLanguage(resolvedLanguage, callback);
  };
  changeLanguagePatched = true;
};

export const initializeI18n = () => {
  if (initializePromise) {
    return initializePromise;
  }

  initializePromise = (async () => {
    const initialLanguage = getInitialLanguage();
    let effectiveInitialLanguage = initialLanguage;
    const initialLanguages = Array.from(
      new Set([DEFAULT_LANGUAGE, initialLanguage]),
    );
    let resourceEntries;
    try {
      resourceEntries = await Promise.all(
        initialLanguages.map(importLocaleResource),
      );
    } catch (error) {
      console.error(
        '[i18n] failed to load initial locale, using fallback',
        error,
      );
      resourceEntries = [await importLocaleResource(DEFAULT_LANGUAGE)];
      effectiveInitialLanguage = DEFAULT_LANGUAGE;
    }
    const resources = resourceEntries.reduce((acc, { language, resource }) => {
      acc[language] = {
        [DEFAULT_NAMESPACE]: resource,
      };
      loadedLanguages.add(language);
      return acc;
    }, {});

    await i18n
      .use(LanguageDetector)
      .use(initReactI18next)
      .init({
        lng: effectiveInitialLanguage,
        load: 'currentOnly',
        supportedLngs: supportedLanguages,
        resources,
        fallbackLng: DEFAULT_LANGUAGE,
        nsSeparator: false,
        interpolation: {
          escapeValue: false,
        },
      });

    patchChangeLanguage();

    return i18n;
  })();

  return initializePromise;
};

export default i18n;
