export default {
  changeTypes: [
    {
      title: '\U0001f4a5 Breaking changes',
      labels: ['breaking'],
      bump: 'major',
      weight: 3,
    },
    {
      title: '\U0001f512 Security',
      labels: ['security'],
      bump: 'patch',
      weight: 2,
    },
    {
      title: '\u2728 Features',
      labels: ['feature', 'feature \U0001f680\ufe0f'],
      bump: 'minor',
      weight: 1,
    },
    {
      title: '\U0001f4c8 Enhancement',
      labels: ['enhancement', 'refactor', 'enhancement \U0001f446\ufe0f'],
      bump: 'minor',
    },
    {
      title: '\U0001f41b Bug Fixes',
      labels: ['bug', 'bug \U0001f41b\ufe0f'],
      bump: 'patch',
    },
    {
      title: '\U0001f4da Documentation',
      labels: ['docs', 'documentation', 'documentation \U0001f4d6\ufe0f'],
      bump: 'patch',
    },
    {
      title: '\U0001f4e6\ufe0f Dependency',
      labels: ['dependency', 'dependencies'],
      bump: 'patch',
      weight: -1,
    },
    {
      title: 'Misc',
      labels: ['misc', 'chore \U0001f9f0'],
      bump: 'patch',
      default: true,
      weight: -2,
    },
  ],
};
