/** 图片加载失败时的统一占位图（内联 SVG，中性底色 + 图片图标轮廓）。 */
export const IMAGE_FALLBACK =
  'data:image/svg+xml;utf8,' +
  encodeURIComponent(
    '<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64">' +
      '<rect width="64" height="64" fill="#f0f2f5"/>' +
      '<path d="M18 22a3 3 0 0 1 3-3h22a3 3 0 0 1 3 3v20a3 3 0 0 1-3 3H21a3 3 0 0 1-3-3z" fill="none" stroke="#bfbfbf" stroke-width="2"/>' +
      '<circle cx="26" cy="28" r="3" fill="#bfbfbf"/>' +
      '<path d="M20 42l8-8 6 6 6-7 8 9" fill="none" stroke="#bfbfbf" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>' +
    '</svg>',
  );
