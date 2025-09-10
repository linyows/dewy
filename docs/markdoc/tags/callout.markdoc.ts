import {Callout} from '../../components';

export const callout = {
  render: Callout,
  children: ['paragraph', 'tag', 'list'],
  attributes: {
    type: {
      type: String,
      default: 'note',
      matches: ['tip', 'note', 'important', 'warning', 'caution'],
      errorLevel: 'critical'
    },
    title: {
      type: String,
    },
  },
};
