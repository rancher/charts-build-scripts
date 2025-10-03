package auto

// func Test_push(t *testing.T) {
// 	type input struct {
// 		o       *oci
// 		release options.ReleaseOptions
// 	}
// 	type expected struct {
// 		pushedAssets []string
// 		err          error
// 	}

// 	tests := []struct {
// 		name     string
// 		input    input
// 		expected expected
// 	}{
// 		{
// 			name: "Test #1",
// 			input: input{
// 				o: &oci{
// 					DNS:        "######",
// 					user:       "######",
// 					password:   "######",
// 					helmClient: &registry.Client{},
// 					loadAsset: func(chart, asset string) ([]byte, error) {
// 						return []byte{}, nil
// 					},
// 					checkAsset: func(ctx context.Context, regClient *registry.Client, ociDNS, chart, version string) (bool, error) {
// 						return false, nil
// 					},
// 					push: func(helmClient *registry.Client, data []byte, url string) error {
// 						return nil
// 					},
// 				},
// 				release: options.ReleaseOptions{
// 					"chart1": {"1.0.0"},
// 				},
// 			},
// 			expected: expected{
// 				pushedAssets: []string{"chart1-1.0.0.tgz"},
// 				err:          nil,
// 			},
// 		},
// 		{
// 			name: "Test #2",
// 			input: input{
// 				o: &oci{
// 					DNS:        "######",
// 					user:       "######",
// 					password:   "######",
// 					helmClient: &registry.Client{},
// 					loadAsset: func(chart, asset string) ([]byte, error) {
// 						return []byte{}, nil
// 					},
// 					checkAsset: func(ctx context.Context, regClient *registry.Client, ociDNS, chart, version string) (bool, error) {
// 						return false, nil
// 					},
// 					push: func(helmClient *registry.Client, data []byte, url string) error {
// 						return nil
// 					},
// 				},
// 				release: options.ReleaseOptions{
// 					"chart1": {"1.0.0+up0.0.0"},
// 					"chart2": {"1.0.0+up0.0.0"},
// 					"chart3": {"1.0.0+up0.0.0"},
// 				},
// 			},
// 			expected: expected{
// 				pushedAssets: []string{
// 					"chart1-1.0.0+up0.0.0.tgz",
// 					"chart2-1.0.0+up0.0.0.tgz",
// 					"chart3-1.0.0+up0.0.0.tgz",
// 				},
// 				err: nil,
// 			},
// 		},
// 		{
// 			name: "Test #3",
// 			input: input{
// 				o: &oci{
// 					DNS:        "######",
// 					user:       "######",
// 					password:   "######",
// 					helmClient: &registry.Client{},
// 					loadAsset: func(chart, asset string) ([]byte, error) {
// 						return []byte{}, errors.New("some-error")
// 					},
// 					checkAsset: func(ctx context.Context, regClient *registry.Client, ociDNS, chart, version string) (bool, error) {
// 						return false, nil
// 					},
// 					push: func(helmClient *registry.Client, data []byte, url string) error {
// 						return nil
// 					},
// 				},
// 				release: options.ReleaseOptions{
// 					"chart1": {"1.0.0+up0.0.0"},
// 				},
// 			},
// 			expected: expected{
// 				pushedAssets: []string{},
// 				err:          errors.New("some-error"),
// 			},
// 		},
// 		{
// 			name: "Test #4",
// 			input: input{
// 				o: &oci{
// 					DNS:        "######",
// 					user:       "######",
// 					password:   "######",
// 					helmClient: &registry.Client{},
// 					loadAsset: func(chart, asset string) ([]byte, error) {
// 						return []byte{}, nil
// 					},
// 					checkAsset: func(ctx context.Context, regClient *registry.Client, ociDNS, chart, version string) (bool, error) {
// 						return false, errors.New("some-error")
// 					},
// 					push: func(helmClient *registry.Client, data []byte, url string) error {
// 						return nil
// 					},
// 				},
// 				release: options.ReleaseOptions{
// 					"chart1": {"1.0.0+up0.0.0"},
// 				},
// 			},
// 			expected: expected{
// 				pushedAssets: []string{},
// 				err:          errors.New("some-error"),
// 			},
// 		},
// 		{
// 			name: "Test #5",
// 			input: input{
// 				o: &oci{
// 					DNS:        "######",
// 					user:       "######",
// 					password:   "######",
// 					helmClient: &registry.Client{},
// 					loadAsset: func(chart, asset string) ([]byte, error) {
// 						return []byte{}, nil
// 					},
// 					checkAsset: func(ctx context.Context, regClient *registry.Client, ociDNS, chart, version string) (bool, error) {
// 						return true, nil
// 					},
// 					push: func(helmClient *registry.Client, data []byte, url string) error {
// 						return nil
// 					},
// 				},
// 				release: options.ReleaseOptions{
// 					"chart1": {"1.0.0+up0.0.0"},
// 				},
// 			},
// 			expected: expected{
// 				pushedAssets: []string{},
// 				err:          nil,
// 			},
// 		},
// 		{
// 			name: "Test #6",
// 			input: input{
// 				o: &oci{
// 					DNS:        "######",
// 					user:       "######",
// 					password:   "######",
// 					helmClient: &registry.Client{},
// 					loadAsset: func(chart, asset string) ([]byte, error) {
// 						return []byte{}, nil
// 					},
// 					checkAsset: func(ctx context.Context, regClient *registry.Client, ociDNS, chart, version string) (bool, error) {
// 						return false, nil
// 					},
// 					push: func(helmClient *registry.Client, data []byte, url string) error {
// 						err := errors.New("some assets failed, please fix and retry only these assets")
// 						return err
// 					},
// 				},
// 				release: options.ReleaseOptions{
// 					"chart1": {"1.0.0+up0.0.0"},
// 				},
// 			},
// 			expected: expected{
// 				pushedAssets: []string{},
// 				err:          errors.New("some assets failed, please fix and retry only these assets"),
// 			},
// 		},
// 	}

// 	for _, test := range tests {
// 		t.Run(test.name, func(t *testing.T) {
// 			assets, err := test.input.o.update(context.Background(), &test.input.release)
// 			if test.expected.err == nil {
// 				if err != nil {
// 					t.Errorf("Expected no error, got: [%v]", err)
// 				}
// 			} else {
// 				if !strings.Contains(err.Error(), test.expected.err.Error()) {
// 					t.Errorf("Expected error: [%v], got: [%v]", test.expected.err, err)
// 				}
// 			}

// 			assert.EqualValues(t, len(assets), len(test.expected.pushedAssets))
// 		})
// 	}

// }
