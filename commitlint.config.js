module.exports = {
  extends: ['@commitlint/config-conventional'],
  rules: {
    // ensure a blank line before body
    'body-leading-blank': [2, 'always'],
    // ensure a blank line before footer
    'footer-leading-blank': [2, 'always'],
    // limit the maximum length of the header
    'header-max-length': [2, 'always', 72],
    // scope enumeration to ensure contributors can only fill in these module names
    'scope-enum': [
      2,
      'always',
      [
        'sliceutil',
        'seqs',
        'lists',
        'queues',
        'set',
        'map',
        'tree',
        'core',
        'deps',
        'release'
      ]
    ],
    // Subject 大小写不做强制限制 (适应中文或英文习惯)
    'subject-case': [0]
  }
};