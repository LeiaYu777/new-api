import React, { useMemo } from 'react';
import DOMPurify from 'dompurify';

const SANITIZE_CONFIG = {
  USE_PROFILES: { html: true },
  FORBID_TAGS: ['script', 'style', 'iframe', 'object', 'embed', 'link', 'meta'],
};

const SafeHtml = ({ html, as = 'div', ...props }) => {
  const Component = as;
  const sanitizedHtml = useMemo(
    () => DOMPurify.sanitize(html || '', SANITIZE_CONFIG),
    [html],
  );
  return <Component {...props} dangerouslySetInnerHTML={{ __html: sanitizedHtml }} />;
};

export default SafeHtml;
