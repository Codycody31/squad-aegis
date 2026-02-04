export default {
  changeTypes: [
    {
      title: '💥 Breaking changes',
      labels: ['breaking'],
      bump: 'major',
      weight: 3,
    },
    {
      title: '🔒 Security',
      labels: ['security'],
      bump: 'patch',
      weight: 2,
    },
    {
      title: '✨ Features',
      labels: ['feature', 'feature 🚀️'],
      bump: 'minor',
      weight: 1,
    },
    {
      title: '📈 Enhancement',
      labels: ['enhancement', 'refactor', 'enhancement 👆️'],
      bump: 'minor',
    },
    {
      title: '🐛 Bug Fixes',
      labels: ['bug', 'bug 🐛️'],
      bump: 'patch',
    },
    {
      title: '📚 Documentation',
      labels: ['docs', 'documentation', 'documentation 📖️'],
      bump: 'patch',
    },
    {
      title: '📦️ Dependency',
      labels: ['dependency', 'dependencies'],
      bump: 'patch',
      weight: -1,
    },
    {
      title: 'Misc',
      labels: ['misc', 'chore 🧰'],
      bump: 'patch',
      default: true,
      weight: -2,
    },
  ],
  commentOnReleasedPullRequests: false,
  skipCommitsWithoutPullRequest: false,
  skipLabels: ['skip-release', 'skip-changelog', 'regression'],
};
